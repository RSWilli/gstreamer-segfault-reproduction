package main

import "github.com/go-gst/go-gst/gst"

// helper to ignore the error on element creation
func Must(el *gst.Element, err error) *gst.Element {
	if err != nil {
		panic(err)
	}

	return el
}

// helper struct to keep the reference to many elements in a bin
type elementChain struct {
	bin *gst.Bin

	els []*gst.Element
}

func (s *elementChain) AddElement(el *gst.Element) error {
	err := s.bin.Add(el)

	if err != nil {
		return err
	}

	s.els = append(s.els, el)

	return nil
}

func (s *elementChain) LastPad() *gst.Pad {
	lastEl := s.els[len(s.els)-1]

	return lastEl.GetStaticPad("src")
}
