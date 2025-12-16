/*************************************************************************
 * Copyright 2018 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package processors

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/gravwell/gcfg"
	"github.com/gravwell/gravwell/v3/ingest/attach"
	"github.com/gravwell/gravwell/v3/ingest/config"
	"github.com/gravwell/gravwell/v3/ingest/entry"
)

const (
	AttachProcessor string = `attach`
)

// AttachLoadConfig loads the configuration for the attach processor
// It converts the VariableConfig to an attach.AttachConfig
func AttachLoadConfig(vc *config.VariableConfig) (c attach.AttachConfig, err error) {
	// The VariableConfig and AttachConfig have the same underlying structure
	// (gcfg.Idxer and Vals map[gcfg.Idx]*[]string)
	c.Idxer = vc.Idxer

	// Copy the Vals map so we can filter without affecting the source config
	c.Vals = make(map[gcfg.Idx]*[]string, len(vc.Vals))
	for k, v := range vc.Vals {
		c.Vals[gcfg.Idx(k)] = v
	}

	// Filter out the "type" key which is used for preprocessor selection
	// but should not be attached as an enumerated value
	delete(c.Vals, c.Idx("type"))

	err = c.Verify()
	return
}

// NewAttachProcessor creates a new attach processor
func NewAttachProcessor(cfg attach.AttachConfig) (*AttachProc, error) {
	if err := cfg.Verify(); err != nil {
		return nil, err
	}
	// Use zero UUID - $UUID is not available in the preprocessor version
	// The ingester UUID is not accessible from the processor creation chain
	id := uuid.UUID{}
	attacher, err := attach.NewAttacher(cfg, id)
	if err != nil {
		return nil, err
	}
	return &AttachProc{
		cfg:      cfg,
		attacher: attacher,
		id:       id,
	}, nil
}

type AttachProc struct {
	nocloser
	cfg      attach.AttachConfig
	attacher *attach.Attacher
	id       uuid.UUID
}

// Config updates the configuration for the attach processor
func (a *AttachProc) Config(v interface{}) (err error) {
	if v == nil {
		err = ErrNilConfig
	} else if cfg, ok := v.(attach.AttachConfig); ok {
		if err = cfg.Verify(); err != nil {
			return
		}
		// Create a new attacher with the updated config
		var attacher *attach.Attacher
		if attacher, err = attach.NewAttacher(cfg, a.id); err != nil {
			return
		}
		a.cfg = cfg
		a.attacher = attacher
	} else {
		err = fmt.Errorf("Invalid configuration, unknown type %T", v)
	}
	return
}

// Process attaches enumerated values to each entry
func (a *AttachProc) Process(ents []*entry.Entry) (rset []*entry.Entry, err error) {
	if len(ents) == 0 {
		return
	}
	rset = ents
	for _, ent := range ents {
		if ent == nil {
			continue
		}
		a.attacher.Attach(ent)
	}
	return
}
