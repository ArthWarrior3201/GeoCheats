package core

import (
	"encoding/binary"
	"errors"
	"math"
)

type SpeedRecord struct {
	Player   uint64  `json:"player"`
	Layer    uint64  `json:"layer"`
	Original float32 `json:"original"`
	Written  float32 `json:"written"`
}

func (s *Session) saveSpeeds() error {
	if s.SpeedJournal == nil {
		return nil
	}
	layer, e := s.layer()
	if e != nil {
		return e
	}
	var records []SpeedRecord
	for ptr, state := range s.players {
		if state.speedOwned && state.layer == layer {
			records = append(records, SpeedRecord{ptr, state.layer, state.speed, state.written})
		}
	}
	return s.SpeedJournal(records)
}

// Reconcile the same running game's saved baseline before resuming speed mods.
// Every pointer, layer identity and current value is checked again before writing.
func (s *Session) RecoverSpeeds(records []SpeedRecord) error {
	if len(records) == 0 {
		return nil
	}
	if s.Image.SHA256 != KnownHash || len(records) > 2 {
		return errors.New("unsupported speed recovery record")
	}
	layer, e := s.layer()
	if e != nil {
		return e
	}
	if layer == 0 {
		return nil
	}
	for _, r := range records {
		if r.Layer != layer {
			continue
		}
		first, e := s.ptr(layer + 0xdb8)
		if e != nil {
			return e
		}
		second, e := s.ptr(layer + 0xdc0)
		if e != nil {
			return e
		}
		if r.Player == 0 || (r.Player != first && r.Player != second) {
			continue
		}
		v, e := s.ptr(r.Player)
		if e != nil {
			return e
		}
		if v != s.Mem.Info().Base+0x7d47c8 {
			continue
		}
		if math.IsNaN(float64(r.Original)) || math.IsNaN(float64(r.Written)) || math.IsInf(float64(r.Original), 0) || math.IsInf(float64(r.Written), 0) || math.Abs(float64(r.Original)) > 20 || math.Abs(float64(r.Written)) > 20 {
			return errors.New("invalid saved speed")
		}
		b, e := s.Mem.Read(r.Player+0x9fc, 4)
		if e != nil {
			return e
		}
		if math.Float32frombits(binary.LittleEndian.Uint32(b)) == r.Written {
			s.players[r.Player] = &playerState{layer: layer, speed: r.Original, written: r.Written, speedOwned: true}
		}
	}
	return s.restoreSpeed()
}
