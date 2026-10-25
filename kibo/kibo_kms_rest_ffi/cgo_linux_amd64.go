//go:build linux && amd64

package kibo_kms_rest_ffi

// #cgo LDFLAGS: ${SRCDIR}/lib/linux_amd64/libkibo_kms_rest_ffi.a -lgcc_s -lutil -lrt -lpthread -lm -ldl -lc[0m
import "C"
