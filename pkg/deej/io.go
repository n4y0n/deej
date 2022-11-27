package deej

import (
	"fmt"

	"go.uber.org/zap"
)

type IO interface {
	Start() error
	Stop()
	SubscribeToSliderMoveEvents() chan SliderMoveEvent
}

func NewIO(deej *Deej, logger *zap.SugaredLogger) (IO, error) {
	switch deej.config.InterfaceType {
	case "serial":
		return NewSerialIO(deej, logger)
	case "tcp":
		return NewNetworkIO(deej, logger)
	default:
		return nil, fmt.Errorf("unknown interface type: %s", deej.config.InterfaceType)
	}
}
