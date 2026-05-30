// SPDX-FileCopyrightText: 2026 go-ft8 authors
// SPDX-License-Identifier: GPL-3.0-only

package ft8

import (
	"math"
	"sync"
)

type refinedCandidate struct {
	FreqHz         float64
	DTSec          float64
	Sync           float64
	HardSync       int
	CostasWins     int
	CostasGeo      float64
	CostasMinBlock float64
}

type downsampler struct {
	realFFT     *realFFTPlan
	complexFFT  *complexFFTPlan
	realInput   []float64
	spectrum    []complex128
	c1          []complex128
	shifted     []complex128
	td          []complex128
	taper       [101]float64
	initialized bool
}

func newDownsampler() *downsampler {
	d := &downsampler{
		realFFT:    newRealFFTPlan(ft8DownsampleFFT1),
		complexFFT: newComplexFFTPlan(ft8DownsampleFFT2),
	}
	for i := range d.taper {
		d.taper[i] = 0.5 * (1.0 + math.Cos(float64(i)*math.Pi/100.0))
	}
	return d
}

var downsamplerPool = sync.Pool{
	New: func() any {
		return newDownsampler()
	},
}

func getDownsampler() *downsampler {
	d := downsamplerPool.Get().(*downsampler)
	d.initialized = false
	return d
}

func putDownsampler(d *downsampler) {
	d.initialized = false
	downsamplerPool.Put(d)
}

func (d *downsampler) downsample(dd []float32, recompute bool, f0 float64) []complex128 {
	if recompute || !d.initialized {
		if d.realInput == nil {
			d.realInput = make([]float64, ft8DownsampleFFT1)
		} else {
			clear(d.realInput)
		}
		for i := 0; i < ft8FrameSamples && i < len(dd); i++ {
			d.realInput[i] = float64(dd[i])
		}
		d.spectrum = d.realFFT.Coefficients(d.spectrum, d.realInput)
		d.initialized = true
	}

	df := float64(wantSampleRate) / float64(ft8DownsampleFFT1)
	baud := float64(wantSampleRate) / float64(ft8SamplesPerSymbol)
	i0 := nint(f0 / df)
	top := min(nint((f0+8.5*baud)/df), ft8DownsampleFFT1/2)
	bottom := max(1, nint((f0-1.5*baud)/df))

	if d.c1 == nil {
		d.c1 = make([]complex128, ft8DownsampleFFT2)
		d.shifted = make([]complex128, ft8DownsampleFFT2)
		d.td = make([]complex128, ft8DownsampleFFT2)
	} else {
		clear(d.c1)
	}
	k := 0
	for i := bottom; i <= top && k < len(d.c1) && i < len(d.spectrum); i++ {
		d.c1[k] = d.spectrum[i]
		k++
	}
	if k > 101 {
		for i := 0; i <= 100; i++ {
			d.c1[i] *= complex(d.taper[100-i], 0)
			d.c1[k-1-100+i] *= complex(d.taper[i], 0)
		}
	}
	shift := i0 - bottom
	shift %= len(d.c1)
	if shift < 0 {
		shift += len(d.c1)
	}
	copied := copy(d.shifted, d.c1[shift:])
	copy(d.shifted[copied:], d.c1[:shift])
	td := d.complexFFT.Sequence(d.td, d.shifted)

	// The backward transform path is intentionally scaled to match the
	// decoder's fixed downsample normalization.
	fac := float64(ft8DownsampleFFT2) / math.Sqrt(float64(ft8DownsampleFFT1*ft8DownsampleFFT2))
	for i := range td {
		td[i] *= complex(fac, 0)
	}
	return td
}

func cshift(in []complex128, shift int) []complex128 {
	n := len(in)
	out := make([]complex128, n)
	if n == 0 {
		return out
	}
	shift %= n
	if shift < 0 {
		shift += n
	}
	for i := range out {
		out[i] = in[(i+shift)%n]
	}
	return out
}

func refineCandidate(dd []float32, cand candidate) refinedCandidate {
	ds := newDownsampler()
	return refineCandidateWithDownsampler(dd, ds, cand, true)
}

func refineCandidateWithDownsampler(dd []float32, ds *downsampler, cand candidate, recompute bool) refinedCandidate {
	refined, _, _ := refineCandidateDetails(dd, ds, cand, recompute)
	return refined
}

func refineCandidateDetails(dd []float32, ds *downsampler, cand candidate, recompute bool) (refinedCandidate, []complex128, int) {
	f1 := cand.FreqHz
	cd0 := ds.downsample(dd, recompute, f1)
	i0 := nint((cand.DTSec + 0.5) * float64(ft8DownsampleRate))

	bestSync := 0.0
	bestIndex := i0
	for idt := i0 - 10; idt <= i0+10; idt++ {
		sync := sync8d(cd0, idt, nil, false)
		if sync > bestSync {
			bestSync = sync
			bestIndex = idt
		}
	}

	bestSync = 0
	bestDelta := 0.0
	for ifr := -5; ifr <= 5; ifr++ {
		delta := float64(ifr) * 0.5
		sync := sync8d(cd0, bestIndex, &toneTweaks[ifr+5], true)
		if sync > bestSync {
			bestSync = sync
			bestDelta = delta
		}
	}
	f1 += bestDelta
	cd0 = ds.downsample(dd, false, f1)

	var finalSyncs [9]float64
	for idt := -4; idt <= 4; idt++ {
		finalSyncs[idt+4] = sync8d(cd0, bestIndex+idt, nil, false)
	}
	bestOffset := 0
	bestSync = finalSyncs[0]
	for i, sync := range finalSyncs {
		if sync > bestSync {
			bestSync = sync
			bestOffset = i
		}
	}
	bestIndex += bestOffset - 4

	_, symbolPower := ds.symbolSpectra(cd0, bestIndex)
	evidence := costasEvidence(symbolPower)
	refined := refinedCandidate{
		FreqHz:         f1,
		DTSec:          float64(bestIndex-1) / float64(ft8DownsampleRate),
		Sync:           bestSync,
		HardSync:       evidence.Wins,
		CostasWins:     evidence.Wins,
		CostasGeo:      evidence.Geo,
		CostasMinBlock: evidence.MinBlock,
	}
	return refined, cd0, bestIndex
}

func toneTweak(deltaHz float64) [32]complex128 {
	var out [32]complex128
	phase := 0.0
	dphase := 2 * math.Pi * deltaHz / float64(ft8DownsampleRate)
	for i := range out {
		out[i] = complex(math.Cos(phase), math.Sin(phase))
		phase = math.Mod(phase+dphase, 2*math.Pi)
	}
	return out
}

var toneTweaks = makeToneTweaks()

func makeToneTweaks() [11][32]complex128 {
	var out [11][32]complex128
	for ifr := -5; ifr <= 5; ifr++ {
		out[ifr+5] = toneTweak(float64(ifr) * 0.5)
	}
	return out
}

var syncWaveforms = makeSyncWaveforms()

func makeSyncWaveforms() [7][32]complex128 {
	var out [7][32]complex128
	for tone := 0; tone < 7; tone++ {
		phase := 0.0
		dphase := 2 * math.Pi * float64(ft8Costas[tone]) / 32.0
		for i := 0; i < 32; i++ {
			out[tone][i] = complex(math.Cos(phase), math.Sin(phase))
			phase = math.Mod(phase+dphase, 2*math.Pi)
		}
	}
	return out
}

func sync8d(cd0 []complex128, i0 int, ctwk *[32]complex128, tweak bool) float64 {
	var sync float64
	for i := 0; i < 7; i++ {
		i1 := i0 + i*32
		i2 := i1 + 36*32
		i3 := i1 + 72*32
		sync += complexPower(syncSum(cd0, i1, i, ctwk, tweak))
		sync += complexPower(syncSum(cd0, i2, i, ctwk, tweak))
		sync += complexPower(syncSum(cd0, i3, i, ctwk, tweak))
	}
	return sync
}

func syncSum(cd0 []complex128, start int, syncTone int, ctwk *[32]complex128, tweak bool) complex128 {
	if start < 0 || start+31 > ft8RefineSamples-1 || start+31 >= len(cd0) {
		return 0
	}
	var z complex128
	for i := 0; i < 32; i++ {
		w := syncWaveforms[syncTone][i]
		if tweak {
			w *= ctwk[i]
		}
		z += cd0[start+i] * complex(real(w), -imag(w))
	}
	return z
}

func complexPower(z complex128) float64 {
	return real(z)*real(z) + imag(z)*imag(z)
}

func tweakFreq1(ca []complex128, npts int, sampleRate float64, a [5]float64) []complex128 {
	if npts > len(ca) {
		npts = len(ca)
	}
	cb := make([]complex128, npts)
	w := complex(1, 0)
	x0 := 0.5 * float64(npts+1)
	scale := 2.0 / float64(npts)
	for i := 1; i <= npts; i++ {
		x := scale * (float64(i) - x0)
		p2 := 1.5*x*x - 0.5
		p3 := 2.5*x*x*x - 1.5*x
		p4 := 4.375*x*x*x*x - 3.75*x*x + 0.375
		dphi := (a[0] + x*a[1] + p2*a[2] + p3*a[3] + p4*a[4]) * (2 * math.Pi / sampleRate)
		w *= complex(math.Cos(dphi), math.Sin(dphi))
		cb[i-1] = w * ca[i-1]
	}
	return cb
}

func symbolSpectra(cd0 []complex128, start int) ([8][ft8Symbols]complex128, [8][ft8Symbols]float64) {
	return newDownsampler().symbolSpectra(cd0, start)
}

func (d *downsampler) symbolSpectra(cd0 []complex128, start int) ([8][ft8Symbols]complex128, [8][ft8Symbols]float64) {
	var cs [8][ft8Symbols]complex128
	var s8 [8][ft8Symbols]float64
	for sym := 0; sym < ft8Symbols; sym++ {
		i1 := start + sym*32
		if i1 >= 0 && i1+31 <= ft8RefineSamples-1 && i1+31 < len(cd0) {
			for tone := 0; tone < 8; tone++ {
				wave := &symbolToneWaveforms[tone]
				var z complex128
				for i := 0; i < 32; i++ {
					z += cd0[i1+i] * wave[i]
				}
				cs[tone][sym] = z / 1000
				s8[tone][sym] = cmplxAbs(z)
			}
		}
	}
	return cs, s8
}

var symbolToneWaveforms = makeSymbolToneWaveforms()

func makeSymbolToneWaveforms() [8][32]complex128 {
	var out [8][32]complex128
	for tone := 0; tone < 8; tone++ {
		phase := 0.0
		dphase := -2 * math.Pi * float64(tone) / 32.0
		for i := 0; i < 32; i++ {
			out[tone][i] = complex(math.Cos(phase), math.Sin(phase))
			phase = math.Mod(phase+dphase, 2*math.Pi)
		}
	}
	return out
}

func hardSync(s8 [8][ft8Symbols]float64) int {
	return costasEvidence(s8).Wins
}

type costasEvidenceResult struct {
	Wins     int
	Geo      float64
	MinBlock float64
}

func costasEvidence(s8 [8][ft8Symbols]float64) costasEvidenceResult {
	var result costasEvidenceResult
	sumLog := 0.0
	count := 0
	minBlockLog := math.Inf(1)
	for k := 0; k < 7; k++ {
		blockLog := 0.0
		for _, sym := range [...]int{k, k + 36, k + 72} {
			ratio, win := costasToneRatio(s8, sym, ft8Costas[k])
			if win {
				result.Wins++
			}
			logRatio := math.Log(ratio)
			sumLog += logRatio
			blockLog += logRatio
			count++
		}
		blockLog /= 3
		if blockLog < minBlockLog {
			minBlockLog = blockLog
		}
	}
	if count > 0 {
		result.Geo = math.Exp(sumLog / float64(count))
	}
	if !math.IsInf(minBlockLog, 0) {
		result.MinBlock = math.Exp(minBlockLog)
	}
	return result
}

func costasToneRatio(s8 [8][ft8Symbols]float64, sym int, targetTone int) (float64, bool) {
	target := s8[targetTone][sym]
	bestOther := 0.0
	for tone := 0; tone < 8; tone++ {
		if tone == targetTone {
			continue
		}
		if s8[tone][sym] > bestOther {
			bestOther = s8[tone][sym]
		}
	}
	if bestOther <= 0 {
		if target > 0 {
			return 1e9, true
		}
		return 1, false
	}
	return target / bestOther, target > bestOther
}

func maxTone(s8 [8][ft8Symbols]float64, sym int) int {
	best := 0
	for tone := 1; tone < 8; tone++ {
		if s8[tone][sym] > s8[best][sym] {
			best = tone
		}
	}
	return best
}

func cmplxAbs(z complex128) float64 {
	x := real(z)
	y := imag(z)
	return math.Sqrt(x*x + y*y)
}
