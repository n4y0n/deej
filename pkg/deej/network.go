package deej

import (
    "errors"
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/zhouhui8915/go-socket.io-client"
    "go.uber.org/zap"

    "github.com/omriharel/deej/pkg/deej/util"
)

type NetworkIO struct {
    ip string
    port uint16

    deej   *Deej
    logger *zap.SugaredLogger

    stopChannel chan bool
    connected   bool
    connOptions NConnectionOptions
    conn        *socketio_client.Client

    lastKnownNumSliders        int
    currentSliderPercentValues []float32

    sliderMoveConsumers []chan SliderMoveEvent
}

type NConnectionOptions struct {
    Ip string
    Port uint16
    Options socketio_client.Options
}

// NewNetworkIO creates a NetworkIO instance that uses the provided deej
// instance's connection info to establish communications with the esp8266 via wireless websockets
func NewNetworkIO(deej *Deej, logger *zap.SugaredLogger) (*NetworkIO, error) {
    logger = logger.Named("network")

    nio := &NetworkIO{
        deej:                deej,
        logger:              logger,
        stopChannel:         make(chan bool),
        connected:           false,
        conn:                nil,
        sliderMoveConsumers: []chan SliderMoveEvent{},
    }

        logger.Debug("Created network i/o instance")

    // respond to config changes
    nio.setupOnConfigReload()

    return nio, nil
}

// Start attempts to connect to our arduino chip
func (sio *NetworkIO) Start() error {

    // don't allow multiple concurrent connections
    if sio.connected {
        sio.logger.Warn("Already connected, can't start another without closing first")
        return errors.New("serial: connection already active")
    }

    sio.connOptions = NConnectionOptions {
        Ip: sio.deej.config.NConnectionInfo.Ip,
        Port: sio.deej.config.NConnectionInfo.Port,
    }

    sio.connOptions.Options = socketio_client.Options{
        Transport: "websocket",
        Query:     make(map[string]string),
    }

    sio.logger.Debugw("Attempting network connection")
    uri := "http://" + sio.deej.config.NConnectionInfo.Ip + ":" + strconv.Itoa(int(sio.connOptions.Port))

    var err error
    sio.conn, err = socketio_client.NewClient(uri, &sio.connOptions.Options)
    if err != nil {

        // might need a user notification here, TBD
        sio.logger.Warnw("Failed to open network connection", "error", err)
        return fmt.Errorf("open network connection: %w", err)
    }

    namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.Options.Transport))

    namedLogger.Infow("Connected", "conn", sio.conn)
    sio.connected = true

    // read lines or await a stop
    sio.conn.On("message", func(msg string) {
        sio.handleLine(namedLogger, msg)
    })
    sio.conn.On("disconnection", func() {
        sio.close(namedLogger)
    })
    sio.conn.On("error", func() {
        if sio.deej.Verbose() {
            namedLogger.Warnw("Error on the socket")
        }
        sio.close(namedLogger)
    })

    return nil
}

// Stop signals us to shut down our serial connection, if one is active
func (sio *NetworkIO) Stop() {
    if sio.connected {
        sio.logger.Debug("Shutting down serial connection")
        sio.stopChannel <- true
    } else {
        sio.logger.Debug("Not currently connected, nothing to stop")
    }
}

// SubscribeToSliderMoveEvents returns an unbuffered channel that receives
// a sliderMoveEvent struct every time a slider moves
func (sio *NetworkIO) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
    ch := make(chan SliderMoveEvent)
    sio.sliderMoveConsumers = append(sio.sliderMoveConsumers, ch)

    return ch
}

func (sio *NetworkIO) setupOnConfigReload() {
    configReloadedChannel := sio.deej.config.SubscribeToChanges()

    const stopDelay = 50 * time.Millisecond

    go func() {
        for {
            select {
            case <-configReloadedChannel:

                // make any config reload unset our slider number to ensure process volumes are being re-set
                // (the next read line will emit SliderMoveEvent instances for all sliders)\
                // this needs to happen after a small delay, because the session map will also re-acquire sessions
                // whenever the config file is reloaded, and we don't want it to receive these move events while the map
                // is still cleared. this is kind of ugly, but shouldn't cause any issues
                go func() {
                    <-time.After(stopDelay)
                    sio.lastKnownNumSliders = 0
                }()

                // if connection params have changed, attempt to stop and start the connection
                if sio.deej.config.NConnectionInfo.Ip != sio.connOptions.Ip || sio.deej.config.NConnectionInfo.Port != sio.connOptions.Port {

                    sio.logger.Info("Detected change in connection parameters, attempting to renew connection")
                    sio.Stop()

                    // let the connection close
                    <-time.After(stopDelay)

                    if err := sio.Start(); err != nil {
                        sio.logger.Warnw("Failed to renew connection after parameter change", "error", err)
                    } else {
                        sio.logger.Debug("Renewed connection successfully")
                    }
                    }
            }
        }
    }()
}

func (sio *NetworkIO) close(logger *zap.SugaredLogger) {
    if err := sio.conn.Emit("disconnection"); err != nil {
        logger.Warnw("Failed to close network connection", "error", err)
    } else {
        logger.Debug("Socket connection closed")
    }

    sio.conn = nil
    sio.connected = false
}

func (sio *NetworkIO) handleLine(logger *zap.SugaredLogger, line string) {

    // this function receives an unsanitized line which is guaranteed to end with LF,
    // but most lines will end with CRLF. it may also have garbage instead of
    // deej-formatted values, so we must check for that! just ignore bad ones
    if !expectedLinePattern.MatchString(line) {
        return
    }

    // trim the suffix
    line = strings.TrimSuffix(line, "\r\n")

    // split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
    splitLine := strings.Split(line, "|")
    numSliders := len(splitLine)

    // update our slider count, if needed - this will send slider move events for all
    if numSliders != sio.lastKnownNumSliders {
        logger.Infow("Detected sliders", "amount", numSliders)
        sio.lastKnownNumSliders = numSliders
        sio.currentSliderPercentValues = make([]float32, numSliders)

        // reset everything to be an impossible value to force the slider move event later
        for idx := range sio.currentSliderPercentValues {
            sio.currentSliderPercentValues[idx] = -1.0
        }
    }

    // for each slider:
    moveEvents := []SliderMoveEvent{}
    for sliderIdx, stringValue := range splitLine {

        // convert string values to integers ("1023" -> 1023)
        number, _ := strconv.Atoi(stringValue)

        // turns out the first line could come out dirty sometimes (i.e. "4558|925|41|643|220")
        // so let's check the first number for correctness just in case
        if sliderIdx == 0 && number > 1023 {
            sio.logger.Debugw("Got malformed line from serial, ignoring", "line", line)
            return
        }

        // map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
        dirtyFloat := float32(number) / 1023.0

        // normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
        normalizedScalar := util.NormalizeScalar(dirtyFloat)

        // if sliders are inverted, take the complement of 1.0
        if sio.deej.config.InvertSliders {
            normalizedScalar = 1 - normalizedScalar
        }

        // check if it changes the desired state (could just be a jumpy raw slider value)
        if util.SignificantlyDifferent(sio.currentSliderPercentValues[sliderIdx], normalizedScalar, sio.deej.config.NoiseReductionLevel) {

            // if it does, update the saved value and create a move event
            sio.currentSliderPercentValues[sliderIdx] = normalizedScalar

            moveEvents = append(moveEvents, SliderMoveEvent{
                SliderID:     sliderIdx,
                PercentValue: normalizedScalar,
                })

            if sio.deej.Verbose() {
                logger.Debugw("Slider moved", "event", moveEvents[len(moveEvents)-1])
            }
        }
    }

    // deliver move events if there are any, towards all potential consumers
    if len(moveEvents) > 0 {
        for _, consumer := range sio.sliderMoveConsumers {
            for _, moveEvent := range moveEvents {
                consumer <- moveEvent
            }
        }
    }
}
