// metrics.go — Symbol spectra and soft-metric extraction for the research package.
//
// Port of the spectra/metric blocks in ft8b.f90 lines 154–239.
//
// TODO: port from Fortran — currently delegates to production ft8x.

package research

import (
	ft8x "github.com/ColonelBlimp/go-ft8"
)

// ComputeSymbolSpectra extracts the complex and magnitude spectra for all
// NN=79 channel symbols from the downsampled signal cd0, starting at
// sample offset ibest.
//
// Port of ft8b.f90 lines 154–161:
//
//	do k=1,NN
//	  i1=ibest+(k-1)*32
//	  csymb=cd0(i1:i1+31)
//	  call four2a(csymb,32,1,-1,1)
//	  cs(0:7,k)=csymb(1:8)/1e3
//	  s8(0:7,k)=abs(csymb(1:8))
//	enddo
//
// TODO: port from Fortran
func ComputeSymbolSpectra(cd0 []complex128, ibest int) ([8][NN]complex128, [8][NN]float64) {
	return ft8x.ComputeSymbolSpectra(cd0, ibest)
}

// ComputeSoftMetrics computes the four sets of soft-decision metrics
// (bmeta, bmetb, bmetc, bmetd) for the 174 LDPC LLR values.
//
// Port of ft8b.f90 lines 182–239 (the nsym=1,2,3 metric extraction).
//
// TODO: port from Fortran
func ComputeSoftMetrics(cs *[8][NN]complex128) (bmeta, bmetb, bmetc, bmetd [174]float64) {
	return ft8x.ComputeSoftMetrics(cs)
}
