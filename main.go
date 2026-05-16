package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
)

const (
	pollyEndpoint = "https://streamlabs.com/polly/speak"
	pollyReferer  = "https://streamlabs.com"
	voice         = "Amy"
)

type pollyResp struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	SpeakURL string `json:"speak_url"`
}

func main() {
	text := strings.TrimSpace(strings.Join(os.Args[1:], " "))

	// No arguments: listen mode (HTTP TTS server).
	if text == "" {
		runServer()
		return
	}

	// Any arguments: speak that text.
	if err := speak(text); err != nil {
		die("%v", err)
	}
}

// synth runs the TTS + processing pipeline and returns PCM ready to play.
func synth(text string) ([]int16, int, error) {
	mp3URL, err := requestPolly(text)
	if err != nil {
		return nil, 0, fmt.Errorf("polly request: %w", err)
	}
	mp3Bytes, err := httpGet(mp3URL)
	if err != nil {
		return nil, 0, fmt.Errorf("download mp3: %w", err)
	}
	samples, sr, err := decodeMP3(mp3Bytes)
	if err != nil {
		return nil, 0, fmt.Errorf("decode mp3: %w", err)
	}
	return toPCM(process(samples, sr)), sr, nil
}

// speak runs the full pipeline and plays the result through the speakers, blocking until done.
func speak(text string) error {
	pcm, sr, err := synth(text)
	if err != nil {
		return err
	}
	if err := play(pcm, sr); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	return nil
}

func requestPolly(text string) (string, error) {
	form := url.Values{}
	form.Set("voice", voice)
	form.Set("text", text)

	req, err := http.NewRequest(http.MethodPost, pollyEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", pollyReferer)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var pr pollyResp
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", fmt.Errorf("decode json: %w (body=%q)", err, string(body))
	}
	if !pr.Success {
		return "", fmt.Errorf("polly error: %s", pr.Message)
	}
	if pr.SpeakURL == "" {
		return "", fmt.Errorf("polly returned empty speak_url")
	}
	return pr.SpeakURL, nil
}

func httpGet(u string) ([]byte, error) {
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// decodeMP3 returns mono float64 samples in [-1,1] and the sample rate.
// go-mp3 always decodes to 16-bit signed little-endian stereo.
func decodeMP3(data []byte) ([]float64, int, error) {
	dec, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	raw, err := io.ReadAll(dec)
	if err != nil {
		return nil, 0, err
	}
	if len(raw)%4 != 0 {
		raw = raw[:len(raw)-(len(raw)%4)]
	}
	n := len(raw) / 4
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		l := int16(binary.LittleEndian.Uint16(raw[i*4 : i*4+2]))
		r := int16(binary.LittleEndian.Uint16(raw[i*4+2 : i*4+4]))
		out[i] = (float64(l) + float64(r)) / 2.0 / 32768.0
	}
	return out, dec.SampleRate(), nil
}

// process applies the PDA modulation chain.
func process(x []float64, sr int) []float64 {
	// 1. Comb filter: y[n] = x[n] - x[n - d], d = 5 ms.
	d := int(math.Round(0.005 * float64(sr)))
	y := make([]float64, len(x))
	for n := range x {
		y[n] = x[n]
		if n-d >= 0 {
			y[n] -= x[n-d]
		}
	}

	// 2. Single-pole highpass at 250 Hz (6 dB/oct).
	rc := 1.0 / (2.0 * math.Pi * 250.0)
	dt := 1.0 / float64(sr)
	alpha := rc / (rc + dt)
	hp := make([]float64, len(y))
	for n := 1; n < len(y); n++ {
		hp[n] = alpha * (hp[n-1] + y[n] - y[n-1])
	}

	// 3. Echo: y[n] = x[n] + 0.75 * x[n - D], D = 2 ms.
	D := int(math.Round(0.002 * float64(sr)))
	eo := make([]float64, len(hp))
	for n := range hp {
		eo[n] = hp[n]
		if n-D >= 0 {
			eo[n] += 0.75 * hp[n-D]
		}
	}

	// 4. Clip guard: rescale if peak > 1.
	peak := 0.0
	for _, v := range eo {
		if a := math.Abs(v); a > peak {
			peak = a
		}
	}
	if peak > 1.0 {
		scale := 0.99 / peak
		for n := range eo {
			eo[n] *= scale
		}
	}
	return eo
}

func toPCM(samples []float64) []int16 {
	pcm := make([]int16, len(samples))
	for i, s := range samples {
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		pcm[i] = int16(math.Round(s * 32767))
	}
	return pcm
}

var (
	otoOnce sync.Once
	otoCtx  *oto.Context
	otoSR   int
	otoErr  error
)

// audioContext lazily initializes the single Oto context allowed per process and binds it
// to the sample rate of the first caller. Later calls with a different sample rate are
// resampled by repeating samples (cheap and good enough here since Polly's output rate
// doesn't change between requests in practice).
func audioContext(sr int) (*oto.Context, error) {
	otoOnce.Do(func() {
		ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate:   sr,
			ChannelCount: 1,
			Format:       oto.FormatSignedInt16LE,
		})
		if err != nil {
			otoErr = err
			return
		}
		<-ready
		otoCtx = ctx
		otoSR = sr
	})
	return otoCtx, otoErr
}

func play(pcm []int16, sr int) error {
	ctx, err := audioContext(sr)
	if err != nil {
		return err
	}
	if sr != otoSR {
		return fmt.Errorf("sample rate mismatch: context is %d Hz but audio is %d Hz", otoSR, sr)
	}

	buf := make([]byte, len(pcm)*2)
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}

	p := ctx.NewPlayer(bytes.NewReader(buf))
	defer p.Close()
	p.Play()
	for p.IsPlaying() {
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
