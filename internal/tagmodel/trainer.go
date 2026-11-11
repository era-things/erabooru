// Package tagmodel fits a regularized binary classifier to one tag and profile.
package tagmodel

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"

	"gonum.org/v1/gonum/optimize"
)

const (
	Dimensions       = 768
	MinimumPositives = 10
	WeakLimit        = 256
	Version          = 1
	// Margin is measured in balanced-training logits. It is a provisional display cutoff.
	Margin         = 0.1
	regularization = 0.01
	memoryRows     = 8192
)

type Source uint8

const (
	Positive Source = iota
	Rejection
	Unlabeled
)

type example struct {
	x      [Dimensions]float32
	source Source
}

// Dataset keeps ordinary collections in memory. Larger confirmed sets spill to
// a private sequential file so all confirmed feedback still enters every fit.
type Dataset struct {
	rows        []example
	file        *os.File
	cleanupPath string
	writer      *bufio.Writer
	record      [1 + 4*Dimensions]byte
	Counts      [3]int
}

func (d *Dataset) Close() {
	if d.file != nil {
		d.file.Close()
		if d.cleanupPath != "" {
			os.Remove(d.cleanupPath)
		}
	}
}

func (d *Dataset) Add(x []float32, source Source) error {
	if len(x) != Dimensions {
		return nil
	}
	var norm float64
	for _, v := range x {
		norm += float64(v) * float64(v)
	}
	if norm == 0 || math.IsNaN(norm) || math.IsInf(norm, 0) {
		return nil
	}
	row := example{source: source}
	scale := 1 / math.Sqrt(norm)
	for i, v := range x {
		row.x[i] = float32(float64(v) * scale)
	}
	if len(d.rows) == memoryRows && d.file == nil {
		f, err := os.CreateTemp("", "erabooru-tag-fit-*")
		if err != nil {
			return err
		}
		d.file = f
		// Unix unlinks an open file immediately, including on process death.
		// On Windows Close removes it after closing the handle.
		if err := os.Remove(f.Name()); err != nil {
			d.cleanupPath = f.Name()
		}
		d.writer = bufio.NewWriterSize(f, 128*1024)
		for i := range d.rows {
			if err := d.write(&d.rows[i]); err != nil {
				return err
			}
		}
		d.rows = nil
	}
	if d.file == nil {
		d.rows = append(d.rows, row)
	} else if err := d.write(&row); err != nil {
		return err
	}
	d.Counts[source]++
	return nil
}

func (d *Dataset) write(row *example) error {
	d.record[0] = byte(row.source)
	for i, v := range row.x {
		binary.LittleEndian.PutUint32(d.record[1+4*i:], math.Float32bits(v))
	}
	_, err := d.writer.Write(d.record[:])
	return err
}

func (d *Dataset) each(ctx context.Context, visit func(*example)) error {
	if d.file == nil {
		for i := range d.rows {
			if i%64 == 0 && ctx.Err() != nil {
				return ctx.Err()
			}
			visit(&d.rows[i])
		}
		return nil
	}
	if err := d.writer.Flush(); err != nil {
		return err
	}
	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(d.file, 128*1024)
	var row example
	for ctx.Err() == nil {
		_, err := io.ReadFull(reader, d.record[:])
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		row.source = Source(d.record[0])
		for i := range row.x {
			row.x[i] = math.Float32frombits(binary.LittleEndian.Uint32(d.record[1+4*i:]))
		}
		visit(&row)
	}
	return ctx.Err()
}

func (d *Dataset) Eligible() bool {
	return d.Counts[Positive] >= MinimumPositives && d.Counts[Rejection]+d.Counts[Unlabeled] > 0
}

// Both sides have mass 0.5. Rejections have at least 4x the per-example
// influence of weak examples. The entire weak group receives at most the
// rejection group's mass when both exist.
func (d *Dataset) weights() [3]float64 {
	p, n, u := d.Counts[0], d.Counts[1], d.Counts[2]
	weakMass := float64(u)
	if n > 0 {
		weakMass = min(weakMass, 4*float64(n))
	}
	denom := 4*float64(n) + weakMass
	w := [3]float64{0.5 / float64(p), 2 / denom, 0}
	if u > 0 {
		w[2] = 0.5 * weakMass / denom / float64(u)
	}
	return w
}

// Fit uses the signed margin form of logistic loss. The bias is unpenalized.
func (d *Dataset) Fit(ctx context.Context, initial []float64) ([]float64, error) {
	if !d.Eligible() {
		return nil, errors.New("insufficient tag examples")
	}
	if len(initial) != Dimensions+1 {
		initial = make([]float64, Dimensions+1)
	}
	weights := d.weights()
	var evalErr error
	objective := func(params, gradient []float64) float64 {
		var loss float64
		clear(gradient)
		for i, w := range params[:Dimensions] {
			loss += 0.5 * regularization * w * w
			if gradient != nil {
				gradient[i] = regularization * w
			}
		}
		evalErr = d.each(ctx, func(row *example) {
			score := params[Dimensions]
			for j, v := range row.x {
				score += float64(v) * params[j]
			}
			sign := 1.0
			if row.source == Positive {
				sign = -1
			}
			z := sign * score
			exp := math.Exp(-math.Abs(z))
			weight := weights[row.source]
			loss += weight * (max(z, 0) + math.Log1p(exp))
			if gradient != nil {
				sigmoid := 1 / (1 + exp)
				if z < 0 {
					sigmoid *= exp
				}
				factor := weight * sign * sigmoid
				for j, v := range row.x {
					gradient[j] += factor * float64(v)
				}
				gradient[Dimensions] += factor
			}
		})
		return loss
	}
	problem := optimize.Problem{
		Func: func(x []float64) float64 { return objective(x, nil) },
		Grad: func(g, x []float64) { objective(x, g) },
		Status: func() (optimize.Status, error) {
			if ctx.Err() != nil {
				return optimize.Failure, ctx.Err()
			}
			if evalErr != nil {
				return optimize.Failure, evalErr
			}
			return optimize.NotTerminated, nil
		},
	}
	result, err := optimize.Minimize(problem, initial, &optimize.Settings{MajorIterations: 50, FuncEvaluations: 150, GradEvaluations: 150, GradientThreshold: 1e-6, Concurrent: 1}, &optimize.LBFGS{Store: 8})
	if err != nil {
		return nil, err
	}
	if err = result.Status.Err(); err != nil {
		return nil, err
	}
	for _, w := range result.X {
		if math.IsNaN(w) || math.IsInf(w, 0) {
			return nil, errors.New("non-finite classifier")
		}
	}
	return result.X, nil
}
