package notification

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send sends a desktop notification to the user
func Send(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		return sendMacOS(title, message)
	case "linux":
		return sendLinux(title, message)
	case "windows":
		return sendWindows(title, message)
	default:
		// Unsupported platform, just print to console
		fmt.Printf("\n🔔 %s: %s\n", title, message)
		return nil
	}
}

func sendMacOS(title, message string) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s" sound name "default"`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

func sendLinux(title, message string) error {
	// Requires notify-send (libnotify-bin package on Ubuntu/Debian)
	cmd := exec.Command("notify-send", title, message)
	return cmd.Run()
}

func sendWindows(title, message string) error {
	// Use PowerShell to show a notification
	script := fmt.Sprintf(`
		[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
		[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
		[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
		
		$template = @"
		<toast>
			<visual>
				<binding template="ToastText02">
					<text id="1">%s</text>
					<text id="2">%s</text>
				</binding>
			</visual>
		</toast>
"@
		
		$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
		$xml.LoadXml($template)
		$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
		[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Forklift").Show($toast)
	`, title, message)

	cmd := exec.Command("powershell", "-Command", script)
	return cmd.Run()
}
