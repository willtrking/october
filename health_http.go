package october

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func healthHTTPHandler(healthChecks HealthChecks) func(http.ResponseWriter, *http.Request) {

	return func(write http.ResponseWriter, req *http.Request) {
		//start := time.Now().UTC()

		// Increment prometheus health metrics
		//healthCounterRequests.Inc()

		checkResult := healthChecks.RunChecks()

		if checkResult.CanonicalStatus == Health_Error {
			write.WriteHeader(503)
		} else {
			write.WriteHeader(200)
		}

		write.Write(checkResult.json())

		/*if checkResult.CanonicalStatus == Health_Error {
			healthCounterResponses.WithLabelValues("503", checkResult.CanonicalStatus.String()).Inc()
		} else {
			healthCounterResponses.WithLabelValues("200", checkResult.CanonicalStatus.String()).Inc()
		}*/

		//stop := time.Now().UTC()

		//latency := (float64(stop.UnixNano()) - float64(start.UnixNano())) / float64(time.Second)
		//healthLatencySummary.Observe(latency)

	}

}

func healthHTTPGinHandler(healthChecks HealthChecks) gin.HandlerFunc {

	handle := healthHTTPHandler(healthChecks)
	return func(c *gin.Context) {
		fmt.Println("OK")
		handle(c.Writer, c.Request)
	}
}
