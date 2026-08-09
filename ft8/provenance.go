// SPDX-FileCopyrightText: 2026 Marc L. Veary (7Q5MLV)
// SPDX-License-Identifier: GPL-3.0-only

package ft8

// DecodeAlgorithm identifies the decoder that recovered a verified payload.
type DecodeAlgorithm string

const (
	// DecodeAlgorithmUnset is the zero value and does not identify a decoder.
	DecodeAlgorithmUnset DecodeAlgorithm = ""
	// DecodeAlgorithmBP is the belief-propagation LDPC decoder.
	DecodeAlgorithmBP DecodeAlgorithm = "bp"
	// DecodeAlgorithmOSD is the ordered-statistics fallback decoder.
	DecodeAlgorithmOSD DecodeAlgorithm = "osd"
	// DecodeAlgorithmA7 is the adjacent-slot message-sequence decoder.
	DecodeAlgorithmA7 DecodeAlgorithm = "a7"
)

// DecodeProvenance identifies how a verified payload was recovered. APProfile
// and APSource are empty for an unassisted metric decode.
type DecodeProvenance struct {
	Algorithm DecodeAlgorithm
	APProfile string
	APSource  string
}

func provenanceForLDPC(result ldpcResult, apProfile, apSource string) DecodeProvenance {
	var algorithm DecodeAlgorithm
	switch result.Decoder {
	case 1:
		algorithm = DecodeAlgorithmBP
	case 2:
		algorithm = DecodeAlgorithmOSD
	}
	return DecodeProvenance{
		Algorithm: algorithm,
		APProfile: apProfile,
		APSource:  apSource,
	}
}
