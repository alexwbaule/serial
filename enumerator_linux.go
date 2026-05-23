// +build linux

package serial

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PortDetail contains detailed information about a serial port.
type PortDetail struct {
	Name         string
	IsUSB        bool
	VID          string
	PID          string
	SerialNumber string
	Manufacturer string
	Product      string
}

// GetPortsList returns a list of serial port names available on the system.
func GetPortsList() ([]string, error) {
	entries, err := os.ReadDir("/sys/class/tty")
	if err != nil {
		return nil, fmt.Errorf("error reading /sys/class/tty: %w", err)
	}

	var ports []string
	for _, entry := range entries {
		name := entry.Name()
		// Skip /dev/console, /dev/tty, etc.
		if strings.HasPrefix(name, "tty") && !strings.HasPrefix(name, "ttyS") && !strings.HasPrefix(name, "ttyUSB") && !strings.HasPrefix(name, "ttyACM") && !strings.HasPrefix(name, "ttyAMA") && !strings.HasPrefix(name, "ttyTHS") {
			// Check if it has a device symlink (real device)
			devicePath := filepath.Join("/sys/class/tty", name, "device")
			if _, err := os.Stat(devicePath); err == nil {
				ports = append(ports, "/dev/"+name)
			}
			continue
		}
		// Include known serial port types
		if strings.HasPrefix(name, "ttyS") || strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "ttyACM") || strings.HasPrefix(name, "ttyAMA") || strings.HasPrefix(name, "ttyTHS") {
			ports = append(ports, "/dev/"+name)
		}
	}

	return ports, nil
}

// GetDetailedPortsList returns detailed information about all serial ports.
// This is the equivalent of go.bug.st/serial/enumerator.GetDetailedPortsList.
func GetDetailedPortsList() ([]*PortDetail, error) {
	entries, err := os.ReadDir("/sys/class/tty")
	if err != nil {
		return nil, fmt.Errorf("error reading /sys/class/tty: %w", err)
	}

	var ports []*PortDetail
	for _, entry := range entries {
		name := entry.Name()

		// Only include known serial port types
		isSerial := strings.HasPrefix(name, "ttyS") ||
			strings.HasPrefix(name, "ttyUSB") ||
			strings.HasPrefix(name, "ttyACM") ||
			strings.HasPrefix(name, "ttyAMA") ||
			strings.HasPrefix(name, "ttyTHS")

		if !isSerial {
			continue
		}

		detail := &PortDetail{
			Name: "/dev/" + name,
		}

		// Check if this is a USB serial device
		subsystemPath := filepath.Join("/sys/class/tty", name, "device", "subsystem")
	 if target, err := os.Readlink(subsystemPath); err == nil {
		 if strings.HasSuffix(target, "usb") || strings.Contains(target, "usb") {
			 detail.IsUSB = true
		 }
	 }

		// Also check via /sys/class/tty/<name>/device/driver -> if usb
		if !detail.IsUSB {
			// Walk up the device path to find USB info
			devicePath := filepath.Join("/sys/class/tty", name, "device")
			detail.IsUSB = isUSBDevice(devicePath)
		}

		if detail.IsUSB {
			// Find the USB device root to read VID/PID/etc
			usbPath := findUSBDevicePath(filepath.Join("/sys/class/tty", name))
			if usbPath != "" {
				detail.VID = readSysfsFile(filepath.Join(usbPath, "idVendor"))
				detail.PID = readSysfsFile(filepath.Join(usbPath, "idProduct"))
				detail.SerialNumber = readSysfsFile(filepath.Join(usbPath, "serial"))
				detail.Manufacturer = readSysfsFile(filepath.Join(usbPath, "manufacturer"))
				detail.Product = readSysfsFile(filepath.Join(usbPath, "product"))
			}
		}

		ports = append(ports, detail)
	}

	return ports, nil
}

// isUSBDevice walks up the /sys path to determine if the device is USB.
func isUSBDevice(devicePath string) bool {
	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return false
	}
	return strings.Contains(realPath, "/usb")
}

// findUSBDevicePath walks up the sysfs tree to find the USB device directory
// that contains idVendor, idProduct, etc.
func findUSBDevicePath(ttyPath string) string {
	// Resolve the device symlink
	devicePath := filepath.Join(ttyPath, "device")
	realPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return ""
	}

	// Walk up the path looking for a directory with idVendor
	dir := realPath
	for len(dir) > 1 && dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "idVendor")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// readSysfsFile reads a small text file from sysfs and returns its content trimmed.
func readSysfsFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
