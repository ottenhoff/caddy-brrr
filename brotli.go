package caddybrrr

import (
	"fmt"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/encode"
	"github.com/molecule-man/go-brrr"
)

func init() {
	caddy.RegisterModule(Brotli{})
}

// Brotli can create Brotli encoders backed by go-brrr.
type Brotli struct {
	Level *int `json:"level,omitempty"`
}

// CaddyModule returns the Caddy module information.
func (Brotli) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.encoders.br",
		New: func() caddy.Module { return new(Brotli) },
	}
}

// UnmarshalCaddyfile sets up the encoder from Caddyfile tokens.
func (b *Brotli) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume option name
	if !d.NextArg() {
		return nil
	}
	level, err := strconv.Atoi(d.Val())
	if err != nil {
		return err
	}
	b.Level = &level
	return nil
}

const defaultBrotliLevel = 4

// Provision provisions b's configuration.
func (b *Brotli) Provision(ctx caddy.Context) error {
	if b.Level == nil {
		level := defaultBrotliLevel
		b.Level = &level
	}
	return nil
}

// Validate validates b's configuration.
func (b Brotli) Validate() error {
	if *b.Level < brrr.BestSpeed {
		return fmt.Errorf("quality too low; must be >= %d", brrr.BestSpeed)
	}
	if *b.Level > brrr.BestCompression {
		return fmt.Errorf("quality too high; must be <= %d", brrr.BestCompression)
	}
	return nil
}

// AcceptEncoding returns the name of the encoding as used in Accept-Encoding.
func (Brotli) AcceptEncoding() string {
	return "br"
}

// NewEncoder returns a new Brotli writer.
func (b Brotli) NewEncoder() encode.Encoder {
	writer, _ := brrr.NewWriter(nil, *b.Level)
	return writer
}

var (
	_ encode.Encoding       = (*Brotli)(nil)
	_ caddy.Provisioner     = (*Brotli)(nil)
	_ caddy.Validator       = (*Brotli)(nil)
	_ caddyfile.Unmarshaler = (*Brotli)(nil)
)
