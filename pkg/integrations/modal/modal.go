package modal

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/baselinehq/focus-exporter/pkg/integrations"
	"github.com/baselinehq/focus-exporter/pkg/integrations/modal/modalpb"
	"github.com/baselinehq/focus-exporter/pkg/model"
)

const (
	Name     = "modal"
	endpoint = "api.modal.com:443"
)

type reporter func(ctx context.Context, start, end time.Time) ([]*modalpb.WorkspaceBillingReportItem, error)

type source struct {
	report    reporter
	accountID string
}

func New(tokenID, tokenSecret, accountID string) integrations.Source {
	return newSource(grpcReporter(tokenID, tokenSecret), accountID)
}

func newSource(r reporter, accountID string) *source {
	return &source{report: r, accountID: accountID}
}

func (s *source) Name() string { return Name }

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	items, err := s.report(ctx, start, end)
	if err != nil {
		return nil, err
	}
	out := []model.UsageRecord{}
	for _, item := range items {
		if item.GetCost() == "" {
			continue
		}
		out = append(out, s.toRecord(item))
	}
	return out, nil
}

func (s *source) toRecord(item *modalpb.WorkspaceBillingReportItem) model.UsageRecord {
	cost := model.Dec(item.GetCost())
	rec := model.UsageRecord{
		Provider:          "Modal",
		Publisher:         "Modal",
		InvoiceIssuer:     "Modal",
		ServiceName:       "Modal",
		ServiceCategory:   model.ServiceCategoryCompute,
		ChargeCategory:    model.ChargeUsage,
		ChargeFrequency:   model.ChargeFrequencyUsageBased,
		PricingCategory:   model.PricingStandard,
		Currency:          "USD",
		PricingCurrency:   "USD",
		BillingAccountID:  s.accountID,
		Day:               item.GetInterval().AsTime().UTC(),
		Cost:              &cost,
		ResourceID:        item.GetObjectId(),
		ResourceName:      item.GetDescription(),
		ResourceType:      objectType(item.GetObjectId()),
		SkuID:             item.GetObjectId(),
		ChargeDescription: chargeDescription(item),
	}
	if details := stringMap(item.GetCostByResource()); details != nil {
		rec.SkuPriceDetails = details
	}
	rec.Extensions = map[string]any{}
	if env := item.GetEnvironmentName(); env != "" {
		rec.Extensions["x_Environment"] = env
	}
	if tags := stringMap(item.GetTags()); tags != nil {
		rec.Extensions["x_Tags"] = tags
	}
	if len(rec.Extensions) == 0 {
		rec.Extensions = nil
	}
	return rec
}

func chargeDescription(item *modalpb.WorkspaceBillingReportItem) string {
	if d := item.GetDescription(); d != "" {
		return d
	}
	return item.GetObjectId()
}

func objectType(objectID string) string {
	prefix, _, ok := strings.Cut(objectID, "-")
	if !ok {
		return ""
	}
	switch prefix {
	case "ap":
		return "App"
	case "fu":
		return "Function"
	case "vo":
		return "Volume"
	case "im":
		return "Image"
	case "sb":
		return "Sandbox"
	default:
		return ""
	}
}

func stringMap(m map[string]string) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func grpcReporter(tokenID, tokenSecret string) reporter {
	return func(ctx context.Context, start, end time.Time) (items []*modalpb.WorkspaceBillingReportItem, err error) {
		conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		if err != nil {
			return nil, fmt.Errorf("modal: dial: %w", err)
		}
		defer func() {
			if cerr := conn.Close(); cerr != nil && err == nil {
				err = fmt.Errorf("modal: close: %w", cerr)
			}
		}()

		md := metadata.New(map[string]string{
			"x-modal-token-id":     tokenID,
			"x-modal-token-secret": tokenSecret,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		req := &modalpb.WorkspaceBillingReportRequest{
			StartTimestamp: timestamppb.New(start.UTC()),
			EndTimestamp:   timestamppb.New(end.UTC()),
			Resolution:     "d",
		}
		stream, err := modalpb.NewModalClientClient(conn).WorkspaceBillingReport(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("modal: WorkspaceBillingReport: %w", err)
		}

		for {
			item, rerr := stream.Recv()
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				return nil, fmt.Errorf("modal: stream: %w", rerr)
			}
			items = append(items, item)
		}
		return items, nil
	}
}

var Provider = integrations.Provider{
	Name:         Name,
	Capabilities: integrations.Capabilities{RequiresTimeRange: true},
	New: func(_ integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
		tokenID := env("MODAL_TOKEN_ID")
		tokenSecret := env("MODAL_TOKEN_SECRET")
		workspace := env("MODAL_WORKSPACE_ID")
		if tokenID == "" || tokenSecret == "" || workspace == "" {
			return nil, fmt.Errorf("missing MODAL_TOKEN_ID / MODAL_TOKEN_SECRET / MODAL_WORKSPACE_ID env")
		}
		return New(tokenID, tokenSecret, workspace), nil
	},
}
