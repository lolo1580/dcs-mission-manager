package panelservice

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"dcsmanager/internal/hid"
	"dcsmanager/internal/panel"
)

// Service watches the flight panels in the background.
type Service struct {
	opts Options

	// emit publishes an event. It is a field so tests can capture events without
	// a channel.
	emit func(Event)
	// openDevice opens a HID path. It is a field so a test can supply a fake
	// device and exercise the read loop without hardware.
	openDevice func(path string) (readWriter, error)
	// enumerate lists present HID devices, injectable for the same reason.
	enumerate func() ([]hid.DeviceInfo, error)

	mu      sync.Mutex
	devices map[string]*device // by path
	stop    chan struct{}
	done    chan struct{}
	started bool
}

// readWriter is the part of a HID device the service uses. It is an interface so
// the read loop can be tested without hardware.
type readWriter interface {
	Read(buf []byte, timeout time.Duration) (int, error)
	Write(report []byte) (int, error)
	Close() error
	// Info returns the device's descriptors as read when it was opened, which is
	// richer than what enumeration alone can see (the product and manufacturer
	// strings need the device open).
	Info() hid.DeviceInfo
}

// device is one open panel the service is reading.
type device struct {
	path  string
	model panel.Model
	info  DeviceInfo
	dev   readWriter

	// writeMu serialises output reports: the read loop and a command must not
	// interleave writes on the same handle.
	writeMu sync.Mutex

	// last is the previous input report, so changes can be diffed.
	last []byte

	// stop ends this device's read loop.
	stop chan struct{}
	done chan struct{}
}

// New creates a service. emit is called for every event; it must not block.
func New(opts Options, emit func(Event)) *Service {
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = 500 * time.Millisecond
	}
	if emit == nil {
		emit = func(Event) {}
	}
	return &Service{
		opts:       opts,
		emit:       emit,
		openDevice: func(path string) (readWriter, error) { return hid.Open(path) },
		enumerate:  hid.Enumerate,
		devices:    make(map[string]*device),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Start begins watching. It returns immediately; call Stop to finish.
func (s *Service) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.loop()
}

// Stop ends the service and closes every device.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	s.mu.Unlock()

	close(s.stop)
	<-s.done
}

// Devices returns the connected panels, as a snapshot.
func (s *Service) Devices() []DeviceInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DeviceInfo, 0, len(s.devices))
	for _, d := range s.devices {
		out = append(out, d.info)
	}
	return out
}

// Write sends an output report to a connected panel, by device path.
func (s *Service) Write(devicePath string, report []byte) error {
	s.mu.Lock()
	d := s.devices[devicePath]
	s.mu.Unlock()
	if d == nil {
		return fmt.Errorf("panelservice: no device %s", devicePath)
	}
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	_, err := d.dev.Write(report)
	return err
}

// loop rescans for panels on a ticker until stopped.
func (s *Service) loop() {
	defer close(s.done)

	ticker := time.NewTicker(s.opts.PollInterval)
	defer ticker.Stop()

	s.scan()
	for {
		select {
		case <-s.stop:
			s.closeAll()
			return
		case <-ticker.C:
			s.scan()
		}
	}
}

// scan reconciles the connected panels with what the HID layer reports: open the
// new ones, close the gone ones.
func (s *Service) scan() {
	found, err := s.enumerate()
	if err != nil {
		s.publish(Event{Kind: KindError, At: time.Now(), Err: err})
		return
	}

	// Which supported panels are present now.
	present := make(map[string]hid.DeviceInfo)
	for _, d := range found {
		if _, ok := panel.ModelFor(d.VendorID, d.ProductID); !ok {
			continue
		}
		present[d.Path] = d
	}

	// Open what is new.
	for path, info := range present {
		s.mu.Lock()
		_, known := s.devices[path]
		s.mu.Unlock()
		if known {
			continue
		}
		s.open(path, info)
	}

	// Close what is gone.
	s.mu.Lock()
	var gone []string
	for path := range s.devices {
		if _, ok := present[path]; !ok {
			gone = append(gone, path)
		}
	}
	s.mu.Unlock()
	for _, path := range gone {
		s.close(path, "unplugged")
	}
}

// open starts reading one panel.
func (s *Service) open(path string, info hid.DeviceInfo) {
	model, ok := panel.ModelFor(info.VendorID, info.ProductID)
	if !ok {
		return
	}
	dev, err := s.openDevice(path)
	if err != nil {
		// A panel another tool holds exclusively is normal (Logitech's own
		// software, DCSFlightpanels), and must not be reported as a crash.
		s.publish(Event{
			Kind:   KindError,
			At:     time.Now(),
			Device: path,
			Model:  model,
			Err:    fmt.Errorf("opening %s: %w", path, err),
		})
		return
	}

	// Prefer the descriptors read when opening: enumeration does not open the
	// device, so it cannot see the product and manufacturer strings. Falling back
	// to the enumeration data keeps a device whose strings are empty usable.
	opened := dev.Info()
	if opened.Product == "" {
		opened.Product = info.Product
	}
	if opened.Manufacturer == "" {
		opened.Manufacturer = info.Manufacturer
	}
	if opened.Serial == "" {
		opened.Serial = info.Serial
	}

	d := &device{
		path:  path,
		model: model,
		info: DeviceInfo{
			Path:         path,
			VendorID:     info.VendorID,
			ProductID:    info.ProductID,
			Product:      opened.Product,
			Manufacturer: opened.Manufacturer,
			Serial:       opened.Serial,
		},
		dev:  dev,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}

	s.mu.Lock()
	s.devices[path] = d
	s.mu.Unlock()

	s.publish(Event{Kind: KindConnected, At: time.Now(), Device: path, Model: model, Info: d.info})
	go s.readLoop(d)
}

// close stops a device's reader and forgets it.
func (s *Service) close(path, why string) {
	s.mu.Lock()
	d := s.devices[path]
	delete(s.devices, path)
	s.mu.Unlock()
	if d == nil {
		return
	}
	close(d.stop)
	<-d.done
	d.dev.Close()
	s.publish(Event{Kind: KindDisconnected, At: time.Now(), Device: path, Model: d.model})
	log.Printf("panelservice: %s %s", d.model, why)
}

func (s *Service) closeAll() {
	s.mu.Lock()
	paths := make([]string, 0, len(s.devices))
	for path := range s.devices {
		paths = append(paths, path)
	}
	s.mu.Unlock()
	for _, path := range paths {
		s.close(path, "stopping")
	}
}

// readLoop reads one device until it is stopped. A read error closes the device
// rather than spinning: a handle that fails usually keeps failing.
func (s *Service) readLoop(d *device) {
	defer close(d.done)

	buf := make([]byte, 64)
	failures := 0
	for {
		select {
		case <-d.stop:
			return
		default:
		}

		n, err := d.dev.Read(buf, s.opts.ReadTimeout)
		if errors.Is(err, hid.ErrTimeout) {
			continue // idle panel: nothing to report, not a failure
		}
		if err != nil {
			failures++
			if failures >= 3 {
				s.publish(Event{
					Kind:   KindError,
					At:     time.Now(),
					Device: d.path,
					Model:  d.model,
					Err:    fmt.Errorf("reading %s: %w", d.path, err),
				})
				// Ask the scanner to drop it; it will be reopened if it is still
				// there and healthy again.
				s.mu.Lock()
				delete(s.devices, d.path)
				s.mu.Unlock()
				d.dev.Close()
				return
			}
			continue
		}

		report := make([]byte, n)
		copy(report, buf[:n])

		events := panel.Decode(d.path, d.model, d.last, report)
		d.last = report
		for _, e := range events {
			s.publish(Event{Kind: KindInput, At: time.Now(), Device: d.path, Model: d.model, Input: e})
		}
	}
}

func (s *Service) publish(e Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	s.emit(e)
}
