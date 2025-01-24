package util

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ebfe/scard"
	"github.com/varokas/tis620"
)

// EstablishContext initializes the PC/SC context.
func EstablishContext() (*scard.Context, error) {
	return scard.EstablishContext()
}

// ReleaseContext releases the PC/SC context.
func ReleaseContext(ctx *scard.Context) {
	ctx.Release()
}

// ListReaders lists all available smart card readers.
func ListReaders(ctx *scard.Context) ([]string, error) {
	return ctx.ListReaders()
}

// InitReaderStates initializes the reader states for status monitoring.
func InitReaderStates(readers []string) []scard.ReaderState {
	rs := make([]scard.ReaderState, len(readers))
	for i := range rs {
		rs[i].Reader = readers[i]
		rs[i].CurrentState = scard.StateUnaware
	}
	return rs
}

// WaitUntilCardPresent waits for a card to be inserted into the reader.
func WaitUntilCardPresent(ctx *scard.Context, rs []scard.ReaderState) (int, error) {
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			return -1, errors.New("timeout waiting for card to be inserted")
		default:
			err := ctx.GetStatusChange(rs, -1)
			if err != nil {
				return -1, fmt.Errorf("error getting status change: %w", err)
			}

			for i := range rs {
				rs[i].CurrentState = rs[i].EventState
				if rs[i].EventState&scard.StatePresent != 0 {
					log.Println("Card inserted")
					return i, nil
				}
			}
		}
	}
}

// WaitUntilCardRemove waits for a card to be removed from the reader.
func WaitUntilCardRemove(ctx *scard.Context, rs []scard.ReaderState) (int, error) {
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			return -1, errors.New("timeout waiting for card to be removed")
		default:
			err := ctx.GetStatusChange(rs, -1)
			if err != nil {
				return -1, fmt.Errorf("error getting status change: %w", err)
			}

			for i := range rs {
				rs[i].CurrentState = rs[i].EventState
				if rs[i].EventState&scard.StateEmpty != 0 {
					log.Println("Card removed")
					return i, nil
				}
			}
		}
	}
}

// ConnectCard connects to a smart card in the specified reader.
func ConnectCard(ctx *scard.Context, reader string) (*scard.Card, error) {
	maxRetries := 3
	var card *scard.Card
	var err error

	for i := 0; i < maxRetries; i++ {
		// Delay before connecting to allow card stabilization
		time.Sleep(200 * time.Millisecond)

		card, err = ctx.Connect(reader, scard.ShareShared, scard.ProtocolAny)
		if err == nil {
			status, err := card.Status()
			if err == nil {
				stateFlag := scard.StateFlag(status.State)
				if stateFlag&scard.StateUnpowered != 0 {
					log.Println("Card is unpowered, attempting to reset")
					err = card.Reconnect(scard.ShareShared, scard.ProtocolAny, scard.ResetCard)
					if err == nil {
						return card, nil
					}
				} else {
					return card, nil
				}
			}
		}

		if err != nil && i < maxRetries-1 {
			log.Printf("Failed to connect to card (attempt %d/%d): %v. Retrying...", i+1, maxRetries, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}

	return nil, fmt.Errorf("failed to connect to card after %d retries: %w", maxRetries, err)
}

// DisconnectCard disconnects and unpowers the card.
func DisconnectCard(card *scard.Card) error {
	if card == nil {
		return errors.New("card is nil")
	}
	err := card.Disconnect(scard.UnpowerCard)
	if err != nil {
		log.Printf("Error disconnecting card: %v", err)
	}
	return err
}

// GetResponseCommand determines the APDU command based on the ATR value.
func GetResponseCommand(atr []byte) ([]byte, error) {
	if len(atr) < 2 {
		return nil, fmt.Errorf("invalid ATR: %x", atr)
	}
	if atr[0] == 0x3B && atr[1] == 0x67 {
		return []byte{0x00, 0xc0, 0x00, 0x01}, nil
	}
	return []byte{0x00, 0xc0, 0x00, 0x00}, nil
}

// ReadData reads data from the smart card.
func ReadData(card *scard.Card, cmd []byte, cmdGetResponse []byte) (string, error) {
	return readDataToString(card, cmd, cmdGetResponse, false)
}

// ReadDataThai reads data in TIS-620 encoding from the smart card.
func ReadDataThai(card *scard.Card, cmd []byte, cmdGetResponse []byte) (string, error) {
	return readDataToString(card, cmd, cmdGetResponse, true)
}

// readDataToString reads data from the card and converts it to a string.
func readDataToString(card *scard.Card, cmd []byte, cmdGetResponse []byte, isTIS620 bool) (string, error) {
	_, err := card.Status()
	if err != nil {
		return "", fmt.Errorf("card status error: %w", err)
	}

	for i := 0; i < 3; i++ { // Retry logic for transient failures
		_, err = card.Transmit(cmd)
		if err == nil {
			break
		}
		if i == 2 {
			return "", fmt.Errorf("failed to transmit command after 3 retries: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	cmdRespond := append(cmdGetResponse[:], cmd[len(cmd)-1])
	rsp, err := card.Transmit(cmdRespond)
	if err != nil {
		return "", fmt.Errorf("failed to transmit response command: %w", err)
	}

	if len(rsp) < 2 {
		return "", fmt.Errorf("invalid response length: %x", rsp)
	}

	if isTIS620 {
		rsp = tis620.ToUTF8(rsp)
	}

	return strings.TrimSpace(string(rsp[:len(rsp)-2])), nil
}

// ReadLaserData reads laser-engraved data from the card.
func ReadLaserData(card *scard.Card, cmd []byte, cmdGetResponse []byte) (string, error) {
	_, err := card.Status()
	if err != nil {
		return "", fmt.Errorf("card status error: %w", err)
	}

	for i := 0; i < 3; i++ { // Retry logic
		_, err = card.Transmit(cmd)
		if err == nil {
			break
		}
		if i == 2 {
			return "", fmt.Errorf("failed to transmit command after 3 retries: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	cmdRespond := append(cmdGetResponse[:], 0x10)
	rsp, err := card.Transmit(cmdRespond)
	if err != nil {
		return "", fmt.Errorf("failed to transmit response command: %w", err)
	}

	if len(rsp) < 2 {
		return "", fmt.Errorf("invalid response length: %x", rsp)
	}

	return strings.TrimSpace(string(bytes.Trim(rsp[:len(rsp)-2], "\x00"))), nil
}
