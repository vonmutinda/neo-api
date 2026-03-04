package geo

import (
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/oschwald/geoip2-golang/v2"
)

// Lookuper resolves an IP address to approximate geographic coordinates and location names.
// Used for admin money-flow map visualization; raw IPs are not returned to clients.
type Lookuper interface {
	Lookup(ip string) (lat, lon float64, city, country string, ok bool)
}

// NoopLookuper is a no-op implementation that always returns ok=false.
// Use when no GeoIP database is configured.
type NoopLookuper struct{}

func (NoopLookuper) Lookup(ip string) (lat, lon float64, city, country string, ok bool) {
	return 0, 0, "", "", false
}

// Reader wraps a MaxMind GeoLite2-City or GeoIP2-City database.
type Reader struct {
	db   *geoip2.Reader
	mu   sync.RWMutex
}

// NewReader opens a MaxMind GeoIP2/GeoLite2 database at the given path.
// If path is empty or the file cannot be opened, returns nil and the caller
// should use NoopLookuper. Call Close() when done.
func NewReader(path string) (*Reader, error) {
	if path == "" {
		return nil, nil
	}
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, err
	}
	return &Reader{db: db}, nil
}

// Close closes the database. Safe to call if r is nil.
func (r *Reader) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.db != nil {
		err := r.db.Close()
		r.db = nil
		return err
	}
	return nil
}

// Lookup resolves ip to latitude, longitude, city name, and country name.
// ip may be "host" or "host:port"; the port is stripped. Private and invalid
// IPs return ok=false. Only coordinates and location names are returned; the
// raw IP is never exposed to callers.
func (r *Reader) Lookup(ip string) (lat, lon float64, city, country string, ok bool) {
	if r == nil || r.db == nil {
		return 0, 0, "", "", false
	}
	host := stripPort(ip)
	if host == "" {
		return 0, 0, "", "", false
	}
	parsed, err := netip.ParseAddr(host)
	if err != nil {
		return 0, 0, "", "", false
	}
	r.mu.RLock()
	db := r.db
	r.mu.RUnlock()
	if db == nil {
		return 0, 0, "", "", false
	}
	record, err := db.City(parsed)
	if err != nil || record == nil || !record.HasData() {
		return 0, 0, "", "", false
	}
	if record.Location.Latitude != nil {
		lat = *record.Location.Latitude
	}
	if record.Location.Longitude != nil {
		lon = *record.Location.Longitude
	}
	city = record.City.Names.English
	country = record.Country.Names.English
	return lat, lon, city, country, true
}

func stripPort(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		return s
	}
	return host
}
