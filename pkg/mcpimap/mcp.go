package mcpimap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"
)

const mimeTypeJSON = "application/json"

type EmailSummary struct {
	ID       string    `json:"id"`
	Subject  string    `json:"subject"`
	Received time.Time `json:"received"`
	From     []string  `json:"from"`
}

type EmailHead struct {
	ID       string    `json:"id"`
	Subject  string    `json:"subject"`
	Received time.Time `json:"received"`
	From     []string  `json:"from"`
}

type EmailBody struct {
	MimeType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

type EmailBodies []EmailBody

type IMAPClient interface {
	Mailbox() string
	Host() string
	ListEmails(ctx context.Context) (iter.Seq2[EmailSummary, error], error)
	RetrieveEmailHead(ctx context.Context, id string) (EmailHead, error)
	RetrieveEmailBody(ctx context.Context, id string) (EmailBodies, error)
}

type MCPImap struct {
	client IMAPClient
}

func NewIMAPClient(userAccount IMAPClient) *MCPImap {
	return &MCPImap{
		client: userAccount,
	}
}

const uriTemplateRoot = "mcp-imap://{inbox}@{host}/"

func (m *MCPImap) Attach(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		Description: "Discovers available accounts for mcp-imap",
		MIMEType:    mimeTypeJSON,
		Name:        "Lists available IMAP accounts",
		URI:         "mcp-imap:///",
	}, m.handleDiscovery)
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Description: "Retrieves emails for the given user given the account URI",
		MIMEType:    mimeTypeJSON,
		Name:        "Retrieves emails",
		URITemplate: uriTemplateRoot,
	}, m.handleClientRoot)

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Description: "Pulls the summary of the e-mails",
		MIMEType:    mimeTypeJSON,
		Name:        "E-mail summary",
		URITemplate: uriPatternEmailHead,
	}, m.handleClientEmail)

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Description: "Retrieve the bodies of a specific emails",
		Name:        "Retrieve the bodies of a specific emails",
		URITemplate: uriPatternEmailBodies,
	}, m.handleClientEmailBody)
}

func (m *MCPImap) handleDiscovery(ctx context.Context, r *mcp.ReadResourceRequest) (out *mcp.ReadResourceResult, problem error) {
	type Response struct {
		Mailbox string `json:"mailbox"`
		Host    string `json:"host"`
		URI     string `json:"uri"`
	}
	output, err := json.Marshal(Response{
		Mailbox: m.client.Mailbox(),
		Host:    m.client.Host(),
		URI:     fmt.Sprintf("mcp-imap://%s@%s/", m.client.Mailbox(), m.client.Host()),
	})
	if err != nil {
		return nil, err
	}

	out = &mcp.ReadResourceResult{}
	out.Contents = append(out.Contents, &mcp.ResourceContents{
		URI:      fmt.Sprintf("mcp-imap://%s@%s/", m.client.Mailbox(), m.client.Host()),
		MIMEType: mimeTypeJSON,
		Text:     string(output),
	})
	return out, problem
}

func (m *MCPImap) handleClientRoot(ctx context.Context, r *mcp.ReadResourceRequest) (out *mcp.ReadResourceResult, problem error) {
	emails, err := m.client.ListEmails(ctx)
	if err != nil {
		return nil, err
	}
	out = &mcp.ReadResourceResult{}
	for email, err := range emails {
		if err != nil {
			problem = errors.Join(problem, err)
			continue
		}
		body, encodingProblem := json.Marshal(email)
		if encodingProblem != nil {
			problem = errors.Join(problem, encodingProblem)
			continue
		}
		out.Contents = append(out.Contents, &mcp.ResourceContents{
			URI:      fmt.Sprintf("mcp-imap://%s@%s/email/%s", m.client.Mailbox(), m.client.Host(), email.ID),
			MIMEType: mimeTypeJSON,
			Text:     string(body),
		})
	}

	return out, problem
}

const uriPatternEmailHead = "mcp-imap://{inbox}@{host}/email/{email.id}"

func (m *MCPImap) handleClientEmail(ctx context.Context, r *mcp.ReadResourceRequest) (out *mcp.ReadResourceResult, problem error) {
	uri := r.Params.URI
	template := uritemplate.MustNew(uriPatternEmailHead)
	values := template.Match(uri)
	if values == nil {
		return nil, fmt.Errorf("uri %s does not match template", uri)
	}
	emailID := values.Get("email.id").String()
	if emailID == "" {
		return nil, fmt.Errorf("email id not found in uri %s", uri)
	}

	head, err := m.client.RetrieveEmailHead(ctx, emailID)
	if err != nil {
		return nil, err
	}
	body, encodingProblem := json.Marshal(head)
	if encodingProblem != nil {
		return nil, encodingProblem
	}
	out = &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				MIMEType: mimeTypeJSON,
				Text:     string(body),
			},
		},
	}
	return out, problem
}

const uriPatternEmailBodies = "mcp-imap://{inbox}@{host}/email/{email.id}/bodies"

func (m *MCPImap) handleClientEmailBody(ctx context.Context, r *mcp.ReadResourceRequest) (out *mcp.ReadResourceResult, problem error) {
	uri := r.Params.URI
	template := uritemplate.MustNew(uriPatternEmailBodies)
	values := template.Match(uri)
	if values == nil {
		return nil, fmt.Errorf("uri %s does not match template", uri)
	}
	emailID := values.Get("email.id").String()
	if emailID == "" {
		return nil, fmt.Errorf("email id not found in uri %s", uri)
	}

	bodies, err := m.client.RetrieveEmailBody(ctx, emailID)
	if err != nil {
		return nil, err
	}
	out = &mcp.ReadResourceResult{}
	for _, body := range bodies {
		out.Contents = append(out.Contents, &mcp.ResourceContents{
			MIMEType: body.MimeType,
			Blob:     body.Data,
		})
	}
	return out, problem
}
