package ft8x

// Diagnostic test: for each missing signal across all captures, try
// DecodeSingle at the exact WSJT-X freq/DT to determine the failure
// stage (sync, LDPC, unpack, or plausibility).

import (
	"fmt"
	"math"
	"os"
	"testing"
)

type missingSignal struct {
	Label string
	Freq  float64 // Hz
	DT    float64 // xdt (WSJT-X DT - 0.5)
	SNR   int     // WSJT-X reported SNR
}

func TestDiagMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping diagnostic test (slow)")
	}

	type captureInfo struct {
		Name    string
		WAVPath string
		Missing []missingSignal
	}

	captures := []captureInfo{
		{
			Name:    "Capture1",
			WAVPath: "testdata/ft8test_capture_20260410.wav",
			Missing: []missingSignal{
				{"<...> RA1OHX KP91", 2098.6, 0.09 - 0.5, -99},
				{"<...> RA6ABC KN96", 1814.0, -0.2 - 0.5, -99},
				{"ES2AJ UA3LAR KO75", 835.0, -0.1 - 0.5, -99},
				{"HZ1TT RU1AB R-10", 2208.4, 1.08 - 0.5, -99},
				{"<...> RV6ASU KN94", 460.9, 0.18 - 0.5, -99},
			},
		},
		{
			Name:    "Capture2",
			WAVPath: "testdata/ft8test_capture2_20260410.wav",
			Missing: []missingSignal{
				{"UY7VV KE6SU DM14", 553.0, 1.8, -99},
				{"RU4LM 4X5JK R-14", 1840.0, 1.8, -99},
				{"JT1CO IZ7DIO 73", 1768.0, 1.8, -99},
				{"VK3ZSJ US7KC KO21", 1502.0, 1.8, -99},
				{"JR3UIC SP7IIT RR73", 1410.0, 1.8, -99},
				{"JT1CO YO3HST KN24", 1096.0, 1.8, -99},
			},
		},
		{
			Name:    "Capture3",
			WAVPath: "testdata/capture.wav",
			Missing: []missingSignal{
				{"CQ CO8LY FL20", 932.0, 1.2 - 0.5, -8},
				{"5Z4VJ YB1RUS OI33", 1505.0, 1.3 - 0.5, -9},
				{"UA0LW UA4ARH -15", 1211.0, 1.5 - 0.5, -11},
				{"VK3TZ UA3ZNQ KO81", 888.0, 1.6 - 0.5, -19},
				{"VK3TZ RC7O KN87", 963.0, 1.4 - 0.5, -15},
			},
		},
	}

	for _, capt := range captures {
		t.Run(capt.Name, func(t *testing.T) {
			if _, err := os.Stat(capt.WAVPath); os.IsNotExist(err) {
				t.Skipf("WAV not found: %s", capt.WAVPath)
			}
			samples, sr, err := loadWAV(capt.WAVPath)
			if err != nil {
				t.Fatalf("load WAV: %v", err)
			}
			if sr != 12000 {
				t.Fatalf("expected 12000 Hz, got %d", sr)
			}

			ds := NewDownsampler()
			firstCall := true

			for _, sig := range capt.Missing {
				// Try a small grid around the known position: ±5 Hz, ±0.3 s.
				bestStatus := "no_decode"
				bestDetail := ""
				for df := -5.0; df <= 5.0; df += 1.0 {
					for ddt := -0.3; ddt <= 0.3; ddt += 0.1 {
						freq := sig.Freq + df
						dt := sig.DT + ddt

						newdat := firstCall
						firstCall = false

						status, detail := diagDecodeSingle(samples, ds, freq, dt, newdat)
						// Priority: decode > plausibility_fail > ldpc_fail > sync_fail
						if betterStatus(status, bestStatus) {
							bestStatus = status
							bestDetail = detail
						}
					}
				}

				t.Logf("  %-28s %7.1f Hz  SNR=%+3d → %s  %s",
					sig.Label, sig.Freq, sig.SNR, bestStatus, bestDetail)
			}
		})
	}
}

// diagDecodeSingle runs the decode pipeline on a single candidate and
// returns the failure stage.
func diagDecodeSingle(
	dd []float32, ds *Downsampler, f1, xdt float64, newdat bool,
) (status, detail string) {
	cd0 := ds.Downsample(dd, &newdat, f1)

	// Coarse time search.
	i0 := int(math.Round((xdt + 0.5) * Fs2))
	smax := 0.0
	ibest := i0
	var ctwkZero [32]complex128
	for i := range ctwkZero {
		ctwkZero[i] = complex(1, 0)
	}
	for idt := i0 - 10; idt <= i0+10; idt++ {
		sync := Sync8d(cd0, idt, ctwkZero, 0)
		if sync > smax {
			smax = sync
			ibest = idt
		}
	}

	// Fine frequency search.
	smax = 0.0
	delfBest := 0.0
	twopi := 2.0 * math.Pi
	for ifr := -5; ifr <= 5; ifr++ {
		delf := float64(ifr) * 0.5
		dphi := twopi * delf * Dt2
		phi := 0.0
		var ctwk [32]complex128
		for i := 0; i < 32; i++ {
			ctwk[i] = complex(math.Cos(phi), math.Sin(phi))
			phi = math.Mod(phi+dphi, twopi)
		}
		sync := Sync8d(cd0, ibest, ctwk, 1)
		if sync > smax {
			smax = sync
			delfBest = delf
		}
	}

	var a [5]float64
	a[0] = -delfBest
	cd0 = TwkFreq1(cd0, Fs2, a)
	f1 += delfBest

	newdat2 := false
	cd0 = ds.Downsample(dd, &newdat2, f1)

	// Time refinement.
	ss := [9]float64{}
	for idt := -4; idt <= 4; idt++ {
		sync := Sync8d(cd0, ibest+idt, ctwkZero, 0)
		ss[idt+4] = sync
	}
	bestIdx := 0
	for i, v := range ss {
		if v > ss[bestIdx] {
			bestIdx = i
		}
	}
	ibest = ibest + (bestIdx - 4)

	cs, s8 := ComputeSymbolSpectra(cd0, ibest)
	nsync := HardSync(&s8)
	if nsync <= 6 {
		return "sync_fail", fmt.Sprintf("nsync=%d (≤6)", nsync)
	}

	bmeta, bmetb, bmetc, bmetd := ComputeSoftMetrics(&cs)

	// Try all 4 LLR variants with Depth 2 only (Depth 3 OSD is too slow
	// for signals that won't decode).
	metricsAll := [4]*[LDPCn]float64{&bmeta, &bmetb, &bmetc, &bmetd}

	for mi, m := range metricsAll {
		var llrz [LDPCn]float64
		for i, v := range m {
			llrz[i] = ScaleFac * v
		}
		var apmask [LDPCn]int8
		res, ok := Decode174_91(llrz, LDPCk, 1, 2, apmask)
		if !ok || res.NHardErrors < 0 || res.NHardErrors > 36 {
			continue
		}
		var msg77 [77]int8
		copy(msg77[:], res.Message91[:77])
		c77 := BitsToC77(msg77)
		msg, success := Unpack77(c77)
		if !success {
			return "unpack_fail", fmt.Sprintf("m%d nhard=%d", mi, res.NHardErrors)
		}
		if !PlausibleMessage(msg) {
			return "plausibility_fail", fmt.Sprintf("m%d msg=%q nhard=%d", mi, msg, res.NHardErrors)
		}
		return "DECODED", fmt.Sprintf("m%d msg=%q nhard=%d", mi, msg, res.NHardErrors)
	}

	return "ldpc_fail", fmt.Sprintf("nsync=%d", nsync)
}

// betterStatus returns true if a is a more advanced pipeline stage than b.
func betterStatus(a, b string) bool {
	order := map[string]int{
		"no_decode":         0,
		"sync_fail":         1,
		"ldpc_fail":         2,
		"unpack_fail":       3,
		"plausibility_fail": 4,
		"DECODED":           5,
	}
	return order[a] > order[b]
}
