// send_data simulates a sensor device by generating realistic telemetry readings
// (temperature, humidity, co2) and posting them to the IoT ingestion service.
//
// Usage:
//
//	go run scripts/send_data.go \
//	  --url  http://localhost:9080 \
//	  --channel d957bb9f-ebc4-4eec-bb69-086f2e925bbc \
//	  --api-key <your-api-key> \
//	  --interval 5s \
//	  --count 0        # 0 = run forever

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// sensorState holds the simulated "true" values that drift slowly over time.
type sensorState struct {
	temperature float64 // °C
	humidity    float64 // %RH
	co2         float64 // ppm
}

func newSensorState() sensorState {
	return sensorState{
		temperature: 23.0,
		humidity:    60.0,
		co2:         410.0,
	}
}

// drift nudges each sensor value by a small random walk bounded to a realistic range.
func (s *sensorState) drift() {
	s.temperature = clamp(s.temperature+rand.NormFloat64()*0.2, 18.0, 32.0)
	s.humidity = clamp(s.humidity+rand.NormFloat64()*0.5, 30.0, 90.0)
	s.co2 = clamp(s.co2+rand.NormFloat64()*2.0, 350.0, 1500.0)
}

// reading adds a small measurement noise on top of the drifted state.
func (s *sensorState) reading() (temp, hum, co2 float64) {
	temp = round2(s.temperature + rand.NormFloat64()*0.1)
	hum = round2(s.humidity + rand.NormFloat64()*0.3)
	co2 = round2(s.co2 + rand.NormFloat64()*1.0)
	return
}

// --- API payload types -------------------------------------------------------

type ingestRequest struct {
	Fields          map[string]float64    `json:"fields"`
	FieldTimestamps map[string]*time.Time `json:"field_timestamps"`
}

// --- HTTP send ---------------------------------------------------------------

func sendReading(client *http.Client, baseURL, channelID, apiKey string, state *sensorState) error {
	state.drift()

	now := time.Now().UTC()
	temp, hum, co2 := state.reading()

	// Stagger field timestamps by a few seconds, as real sensors would report.
	tTemp := now
	tHum := now.Add(-5 * time.Second)
	tCO2 := now.Add(-10 * time.Second)

	payload := ingestRequest{
		Fields: map[string]float64{
			"temperature": temp,
			"humidity":    hum,
			"co2":         co2,
		},
		FieldTimestamps: map[string]*time.Time{
			"temperature": &tTemp,
			"humidity":    &tHum,
			"co2":         &tCO2,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/v1/channels/%s/data", baseURL, channelID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("warn: read response body: %v", readErr)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, respBody)
	}

	fmt.Printf("[%s] OK  temp=%.2f°C  hum=%.2f%%  co2=%.2fppm\n",
		now.Format(time.RFC3339), temp, hum, co2)
	return nil
}

// --- Main --------------------------------------------------------------------

func main() {
	baseURL := flag.String("url", "http://localhost:9080", "Base server URL")
	channelID := flag.String("channel", "", "Channel ID (required)")
	apiKey := flag.String("api-key", "", "API key (required)")
	interval := flag.Duration("interval", 5*time.Second, "Interval between readings")
	count := flag.Int("count", 0, "Number of readings to send (0 = infinite)")
	flag.Parse()

	if *channelID == "" || *apiKey == "" {
		flag.Usage()
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	state := newSensorState()

	// Catch Ctrl-C for clean shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Starting sensor simulation → %s  channel=%s  interval=%s  count=%d",
		*baseURL, *channelID, *interval, *count)

	sent := 0
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Send first reading immediately, then on each tick.
	for {
		if err := sendReading(client, *baseURL, *channelID, *apiKey, &state); err != nil {
			log.Printf("ERROR: %v", err)
		}
		sent++
		if *count > 0 && sent >= *count {
			log.Printf("Sent %d readings, done.", sent)
			return
		}

		select {
		case <-ticker.C:
		case <-stop:
			log.Printf("Interrupted after %d readings.", sent)
			return
		}
	}
}

// --- helpers -----------------------------------------------------------------

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
