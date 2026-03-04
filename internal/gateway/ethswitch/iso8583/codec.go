package iso8583

import (
	"fmt"

	"github.com/moov-io/iso8583"
)

// Codec handles encoding and decoding of ISO 8583 messages using the
// EthSwitch SmartVista specification.
type Codec struct {
	spec *iso8583.MessageSpec
}

// NewCodec creates a Codec bound to the EthSwitch spec.
func NewCodec() *Codec {
	return &Codec{spec: Spec}
}

// PackAuthRequest marshals an AuthorizationRequest struct into a wire-format
// ISO 8583 byte slice ready to send over the TCP socket.
func (c *Codec) PackAuthRequest(req *AuthorizationRequest) ([]byte, error) {
	msg := iso8583.NewMessage(c.spec)
	if err := msg.Marshal(req); err != nil {
		return nil, fmt.Errorf("marshaling auth request to iso8583: %w", err)
	}

	packed, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("packing iso8583 message: %w", err)
	}

	return packed, nil
}

// UnpackAuthResponse reads a wire-format byte slice and unmarshals it into
// an AuthorizationResponse struct.
func (c *Codec) UnpackAuthResponse(data []byte) (*AuthorizationResponse, error) {
	msg := iso8583.NewMessage(c.spec)
	if err := msg.Unpack(data); err != nil {
		return nil, fmt.Errorf("unpacking iso8583 message: %w", err)
	}

	var resp AuthorizationResponse
	if err := msg.Unmarshal(&resp); err != nil {
		return nil, fmt.Errorf("unmarshaling auth response: %w", err)
	}

	return &resp, nil
}

// PackAuthResponse marshals an AuthorizationResponse into wire format.
// Used when we are the issuer sending a response back to the network.
func (c *Codec) PackAuthResponse(resp *AuthorizationResponse) ([]byte, error) {
	msg := iso8583.NewMessage(c.spec)
	if err := msg.Marshal(resp); err != nil {
		return nil, fmt.Errorf("marshaling auth response to iso8583: %w", err)
	}

	packed, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("packing iso8583 response: %w", err)
	}

	return packed, nil
}

// UnpackAuthRequest reads an incoming authorization request from the network.
func (c *Codec) UnpackAuthRequest(data []byte) (*AuthorizationRequest, error) {
	msg := iso8583.NewMessage(c.spec)
	if err := msg.Unpack(data); err != nil {
		return nil, fmt.Errorf("unpacking iso8583 auth request: %w", err)
	}

	var req AuthorizationRequest
	if err := msg.Unmarshal(&req); err != nil {
		return nil, fmt.Errorf("unmarshaling auth request: %w", err)
	}

	return &req, nil
}
