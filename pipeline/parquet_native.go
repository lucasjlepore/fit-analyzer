//go:build !js

package pipeline

import (
	parquetbuffer "github.com/xitongsys/parquet-go-source/buffer"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

type canonicalParquetRow struct {
	TSUTCISO     string  `parquet:"name=ts_utc_iso, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	ElapsedS     float64 `parquet:"name=elapsed_s, type=DOUBLE"`
	PowerW       float64 `parquet:"name=power_w, type=DOUBLE"`
	HRBPM        float64 `parquet:"name=hr_bpm, type=DOUBLE"`
	CadenceRPM   float64 `parquet:"name=cadence_rpm, type=DOUBLE"`
	SpeedMPS     float64 `parquet:"name=speed_mps, type=DOUBLE"`
	DistanceM    float64 `parquet:"name=distance_m, type=DOUBLE"`
	AltitudeM    float64 `parquet:"name=altitude_m, type=DOUBLE"`
	TemperatureC float64 `parquet:"name=temperature_c, type=DOUBLE"`
	GradePct     float64 `parquet:"name=grade_pct, type=DOUBLE"`
	ValidPower   bool    `parquet:"name=valid_power, type=BOOLEAN"`
	ValidHR      bool    `parquet:"name=valid_hr, type=BOOLEAN"`
	ValidCadence bool    `parquet:"name=valid_cadence, type=BOOLEAN"`
	FileOffset   int64   `parquet:"name=file_offset, type=INT64"`
	RecordIndex  int64   `parquet:"name=record_index, type=INT64"`
}

func marshalCanonicalParquet(samples []CanonicalSample) ([]byte, error) {
	fw := parquetbuffer.NewBufferFile()
	pw, err := writer.NewParquetWriter(fw, new(canonicalParquetRow), 4)
	if err != nil {
		return nil, err
	}
	pw.CompressionType = parquet.CompressionCodec_SNAPPY
	for _, s := range samples {
		row := canonicalParquetRow{
			TSUTCISO:     s.TSUTCISO,
			ElapsedS:     s.ElapsedS,
			PowerW:       valueOrNaN(s.PowerW),
			HRBPM:        valueOrNaN(s.HRBPM),
			CadenceRPM:   valueOrNaN(s.CadenceRPM),
			SpeedMPS:     valueOrNaN(s.SpeedMPS),
			DistanceM:    valueOrNaN(s.DistanceM),
			AltitudeM:    valueOrNaN(s.AltitudeM),
			TemperatureC: valueOrNaN(s.TemperatureC),
			GradePct:     valueOrNaN(s.GradePct),
			ValidPower:   s.ValidPower,
			ValidHR:      s.ValidHR,
			ValidCadence: s.ValidCadence,
			FileOffset:   s.FileOffset,
			RecordIndex:  int64(s.RecordIndex),
		}
		if err := pw.Write(row); err != nil {
			_ = pw.WriteStop()
			return nil, err
		}
	}
	if err := pw.WriteStop(); err != nil {
		return nil, err
	}
	if err := fw.Close(); err != nil {
		return nil, err
	}
	return append([]byte(nil), fw.Bytes()...), nil
}
