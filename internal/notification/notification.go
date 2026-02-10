package notification

import (
	"fmt"

	"github.com/gen2brain/beeep"
)

// Send sends a desktop notification and plays a beep sound
func Send(title, message string) error {
	// Play an explicit beep for testing purposes
	// _ = beeep.Beep(beeep.DefaultFreq, beeep.DefaultDuration)

	// beeep.Alert sends a notification and plays a beep sound
	err := beeep.Alert(title, message, "")
	if err != nil {
		// Fallback to console if notification fails
		fmt.Printf("\n🔔 %s: %s\n", title, message)
		return err
	}
	return nil
}
