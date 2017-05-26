// Command line parsers

package main

import (
	"fmt"
	"image"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// Point parser
type PointValue image.Point

func (p *PointValue) Set(value string) error {
	_, err := fmt.Sscanf(value, "%d,%d", &p.X, &p.Y)
	return err
}

func (p *PointValue) String() string {
	return fmt.Sprintf("%d,%d", p.X, p.Y)
}

func Point(s kingpin.Settings) (target *image.Point) {
	target = &image.Point{}
	s.SetValue((*PointValue)(target))
	return
}

// Rectangle parser
type RectangleValue image.Rectangle

func (r *RectangleValue) Set(value string) error {
	_, err := fmt.Sscanf(value, "%d,%d,%d,%d", &r.Min.X, &r.Min.Y, &r.Max.X, &r.Max.Y)
	return err
}

func (r *RectangleValue) String() string {
	return fmt.Sprintf("%d,%d,%d,%d", r.Min.X, r.Min.Y, r.Max.X, r.Max.Y)
}

func Rectangle(s kingpin.Settings) (target *image.Rectangle) {
	target = &image.Rectangle{}
	s.SetValue((*RectangleValue)(target))
	return
}
