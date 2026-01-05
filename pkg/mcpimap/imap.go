package mcpimap

import (
	"context"
	"fmt"
	"io"
	"iter"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-sasl"
)

type EmersionImap struct {
	mailbox  string
	host     string
	password string
	session  *imapclient.Client
}

func NewEmersionImap(mailbox, host, password string) *EmersionImap {
	return &EmersionImap{
		mailbox:  mailbox,
		host:     host,
		password: password,
	}
}

func (e *EmersionImap) Close(ctx context.Context) error {
	if e.session != nil {
		return e.session.Close()
	}
	return nil
}

func (e *EmersionImap) ensureSession(ctx context.Context) error {
	client, err := imapclient.DialTLS(e.host+":993", nil)
	if err != nil {
		return &operationalError{operation: "dial", problem: err}
	}
	e.session = client

	if err := client.Authenticate(sasl.NewPlainClient(e.mailbox, e.mailbox, e.password)); err != nil {
		return &operationalError{operation: fmt.Sprintf("authentication (%s@%s) error", e.mailbox, e.host), problem: err}
	}
	s := client.Select("INBOX", &imap.SelectOptions{
		ReadOnly: true,
	})
	if _, err := s.Wait(); err != nil {
		return &operationalError{operation: "select INBOX", problem: err}
	}
	return nil
}

func (e *EmersionImap) Mailbox() string {
	return e.mailbox
}

func (e *EmersionImap) Host() string {
	return e.host
}

func (e *EmersionImap) ListEmails(ctx context.Context) (iter.Seq2[EmailSummary, error], error) {
	if err := e.ensureSession(ctx); err != nil {
		return nil, err
	}
	cmd := e.session.Search(&imap.SearchCriteria{
		Since: time.Now().Add(-24 * time.Hour),
	}, &imap.SearchOptions{})
	found, err := cmd.Wait()
	if err != nil {
		return nil, &operationalError{operation: "search", problem: err}
	}

	fetchIterator := e.session.Fetch(found.All, &imap.FetchOptions{
		Envelope: true,
	})
	return func(yield func(EmailSummary, error) bool) {
		//todo: technically we leak the error here
		defer fetchIterator.Close()
		for {
			group := fetchIterator.Next()
			if group == nil {
				return
			}
			for {
				email := group.Next()
				if email == nil {
					break
				}
				switch packet := email.(type) {
				case imapclient.FetchItemDataEnvelope:
					summary := EmailSummary{
						ID:       fmt.Sprintf("%d", group.SeqNum),
						Subject:  packet.Envelope.Subject,
						Received: packet.Envelope.Date,
						From:     []string{packet.Envelope.From[0].Name},
					}
					if !yield(summary, nil) {
						return
					}
				}
			}
		}
	}, nil
}

func (e *EmersionImap) RetrieveEmailHead(ctx context.Context, id string) (EmailHead, error) {
	if err := e.ensureSession(ctx); err != nil {
		return EmailHead{}, err
	}
	var uid imap.UID
	count, err := fmt.Sscanf(id, "%d", &uid)
	if err != nil || count != 1 {
		return EmailHead{}, fmt.Errorf("invalid email id: %s", id)
	}
	return EmailHead{}, nil
}

func (e *EmersionImap) RetrieveEmailBody(ctx context.Context, id string) (EmailBodies, error) {
	var uid imap.UID
	count, err := fmt.Sscanf(id, "%d", &uid)
	if err != nil || count != 1 {
		return nil, fmt.Errorf("invalid email id: %s", id)
	}

	if err := e.ensureSession(ctx); err != nil {
		return EmailBodies{}, err
	}

	bodySection := &imap.FetchItemBodySection{}
	fetch := e.session.Fetch(imap.UIDSetNum(uid), &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{bodySection},
		Envelope:    true,
	})
	output := EmailBodies{}
	for {
		fetchedNext := fetch.Next()
		if fetchedNext == nil {
			break
		}
		for {
			item := fetchedNext.Next()
			if item == nil {
				break
			} else {
				switch item := item.(type) {
				case imapclient.FetchItemDataBodySection:
					output, err = e.consumeEmail(ctx, output, item.Literal)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return output, nil
}

func (e *EmersionImap) consumeEmail(ctx context.Context, data EmailBodies, bodySectionData imap.LiteralReader) (EmailBodies, error) {
	mr, err := mail.CreateReader(bodySectionData)
	if err != nil {
		return nil, err
	}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			return data, nil
		} else if err != nil {
			return data, err
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, err := h.ContentType()
			if err != nil {
				return data, &operationalError{"extracting content header", err}
			}
			b, err := io.ReadAll(p.Body)
			if err != nil {
				return data, &operationalError{"reading body", err}
			}
			data = append(data, EmailBody{
				MimeType: contentType,
				Data:     b,
			})
		}
	}
}
