package audio

import (
	"encoding/base64"
)

// Package audio provides audio format conversion functions

func ConvertMuLawToPCM16kHz(mulaw []byte) []byte {
	// First convert mulaw to PCM
	pcm8k := make([]byte, len(mulaw)*2)
	for i, mulawByte := range mulaw {
		sample := mulawToLinear(mulawByte)
		// Little-endian 16-bit PCM
		pcm8k[i*2] = byte(sample)
		pcm8k[i*2+1] = byte(sample >> 8)
	}
	
	// Upsample from 8kHz to 16kHz (factor of 2)
	return upsamplePCM(pcm8k, 2)
}

func ConvertPCM24kHzToMuLaw8kHz(pcm24k []byte) []byte {
	// First downsample from 24kHz to 8kHz (factor of 3)
	pcm8k := downsamplePCM(pcm24k, 3)
	
	// Then convert to mulaw
	mulaw := make([]byte, len(pcm8k)/2)
	for i := 0; i < len(pcm8k)-1; i += 2 {
		// Get 16-bit PCM sample (little-endian)
		sample := int16(pcm8k[i]) | int16(pcm8k[i+1])<<8
		mulaw[i/2] = linearToMulaw(sample)
	}
	
	return mulaw
}

func ConvertPCM16kHzToMuLaw8kHz(pcm16k []byte) []byte {
	// First downsample from 16kHz to 8kHz (factor of 2)
	pcm8k := downsamplePCM(pcm16k, 2)
	
	// Then convert to mulaw
	mulaw := make([]byte, len(pcm8k)/2)
	for i := 0; i < len(pcm8k)-1; i += 2 {
		// Get 16-bit PCM sample (little-endian)
		sample := int16(pcm8k[i]) | int16(pcm8k[i+1])<<8
		mulaw[i/2] = linearToMulaw(sample)
	}
	
	return mulaw
}

func Base64ToBytes(base64String string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(base64String)
}

func BytesToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func mulawToLinear(mulawByte byte) int16 {
	const BIAS = 0x84
	
	// Invert all bits
	mulawByte = ^mulawByte
	
	// Extract sign, exponent, and mantissa
	sign := mulawByte & 0x80
	exponent := (mulawByte >> 4) & 0x07
	mantissa := mulawByte & 0x0F
	
	// Compute sample
	sample := int16(mantissa<<3 | 0x84)
	sample <<= exponent
	sample -= BIAS
	
	if sign != 0 {
		return -sample
	}
	return sample
}

func linearToMulaw(sample int16) byte {
	const BIAS = 0x84
	const CLIP = 32635
	
	sign := uint8(0)
	if sample < 0 {
		sign = 0x80
		sample = -sample
	}
	
	if sample > CLIP {
		sample = CLIP
	}
	
	sample += BIAS
	
	// Find the position of the most significant bit
	var exponent uint8
	for mask := int16(0x4000); mask != 0 && (sample&mask) == 0; mask >>= 1 {
		exponent++
	}
	
	mantissa := uint8((sample >> (exponent + 3)) & 0x0F)
	exponent = 7 - exponent
	
	return ^(sign | (exponent << 4) | mantissa)
}

func downsamplePCM(pcm []byte, factor int) []byte {
	// Simple downsampling - take every Nth sample
	samples := len(pcm) / 2 // 16-bit samples
	downsampled := make([]byte, (samples/factor)*2)
	
	j := 0
	for i := 0; i < len(pcm)-1; i += 2 * factor {
		if j < len(downsampled)-1 {
			downsampled[j] = pcm[i]
			downsampled[j+1] = pcm[i+1]
			j += 2
		}
	}
	
	return downsampled[:j]
}

func upsamplePCM(pcm []byte, factor int) []byte {
	samples := len(pcm) / 2 // 16-bit samples
	upsampled := make([]byte, samples*factor*2)
	
	for i := 0; i < samples-1; i++ {
		// Get current and next samples
		current := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		next := int16(pcm[(i+1)*2]) | int16(pcm[(i+1)*2+1])<<8
		
		// Linear interpolation between samples
		for j := 0; j < factor; j++ {
			interpolated := current + int16(int32(next-current)*int32(j)/int32(factor))
			idx := (i*factor + j) * 2
			if idx < len(upsampled)-1 {
				upsampled[idx] = byte(interpolated)
				upsampled[idx+1] = byte(interpolated >> 8)
			}
		}
	}
	
	// Handle last sample
	if samples > 0 {
		lastSample := int16(pcm[(samples-1)*2]) | int16(pcm[(samples-1)*2+1])<<8
		for j := 0; j < factor; j++ {
			idx := ((samples-1)*factor + j) * 2
			if idx < len(upsampled)-1 {
				upsampled[idx] = byte(lastSample)
				upsampled[idx+1] = byte(lastSample >> 8)
			}
		}
	}
	
	return upsampled
}