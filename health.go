package october

import (
	"bytes"
	"fmt"
	"time"
)

type HealthCheck interface {
	Name() string
	Description() string
	Check() HealthStatus
}

type HealthStatus uint8

func (h HealthStatus) String() string {
	return string(h.bytes())
}

func (h HealthStatus) bytes() []byte {
	switch h {
	case Health_OK:
		return []byte("OK")
	case Health_Degraded:
		return []byte("Degraded")
	case Health_Error:
		return []byte("Error")
	}

	return []byte("Unknown")
}

const (
	Health_OK HealthStatus = iota
	Health_Degraded
	Health_Error
)

type HealthCheckResult struct {
	Timestamp       time.Time
	CanonicalStatus HealthStatus

	TotalOK       uint64
	TotalDegraded uint64
	TotalError    uint64

	Total uint64

	StatusMap map[string]HealthStatus
}

func (h HealthCheckResult) json() []byte {

	var b bytes.Buffer

	b.WriteString("{\"time\":\"")
	b.WriteString(h.Timestamp.Format(time.RFC3339Nano))
	b.WriteString("\",")

	for n, status := range h.StatusMap {

		b.Write([]byte("\"healthchecks.check."))
		b.WriteString(n)
		b.Write([]byte("\":\""))
		b.Write(status.bytes())
		b.Write([]byte("\","))

	}
	b.Write([]byte("\"healthchecks.total\":"))
	b.WriteString(fmt.Sprintf("%d", h.Total))

	b.Write([]byte(",\"healthchecks.ok\":"))
	b.WriteString(fmt.Sprintf("%d", h.TotalOK))

	b.Write([]byte(",\"healthchecks.degraded\":"))
	b.WriteString(fmt.Sprintf("%d", h.TotalDegraded))

	b.Write([]byte(",\"healthchecks.error\":"))
	b.WriteString(fmt.Sprintf("%d", h.TotalError))

	b.Write([]byte(",\"healthchecks.status\":\""))
	b.Write(h.CanonicalStatus.bytes())
	b.Write([]byte("\"}"))

	return b.Bytes()
}

type HealthChecks map[string]HealthCheck

func (h HealthChecks) AddCheck(name string, check HealthCheck) {
	h[name] = check
}

func (h HealthChecks) RunChecks() HealthCheckResult {
	result := HealthCheckResult{
		Timestamp:       time.Now().UTC(),
		CanonicalStatus: Health_OK,
		Total:           uint64(len(h)),
	}

	// Only do this if we need to
	if len(h) > 0 {
		result.StatusMap = make(map[string]HealthStatus)
	}

	for n, c := range h {

		status := c.Check()
		result.StatusMap[n] = status

		if status == Health_OK {
			result.TotalOK += 1

			if result.CanonicalStatus > status {
				result.CanonicalStatus = status
			}

		} else if status == Health_Degraded {
			result.TotalDegraded += 1

			if result.CanonicalStatus > status {
				result.CanonicalStatus = status
			}

		} else if status == Health_Error {
			result.TotalError += 1

			if result.CanonicalStatus > status {
				result.CanonicalStatus = status
			}

		}
	}

	return result
}
