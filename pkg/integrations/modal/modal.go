package modal

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
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
	Name        = "modal"
	endpoint    = "api.modal.com:443"
	callTimeout = 2 * time.Minute
)

func validAmount(s string) bool {
	if s == "" {
		return false
	}
	_, ok := new(big.Rat).SetString(s)
	return ok
}

type reporter func(ctx context.Context, start, end time.Time) ([]*modalpb.WorkspaceBillingReportItem, error)

type summarizer func(ctx context.Context, start time.Time) (*modalpb.WorkspaceBillingSummaryResponse, error)

type source struct {
	report    reporter
	summarize summarizer
	accountID string
}

func New(_ integrations.HTTPGet, tokenID, tokenSecret, accountID string) integrations.Source {
	return newSource(grpcReporter(tokenID, tokenSecret), grpcSummarizer(tokenID, tokenSecret), accountID)
}

func newSource(r reporter, s summarizer, accountID string) *source {
	return &source{report: r, summarize: s, accountID: accountID}
}

func (s *source) Name() string { return Name }

func (s *source) Fetch(ctx context.Context, start, end time.Time) ([]model.UsageRecord, error) {
	items, err := s.report(ctx, start, end)
	if err != nil {
		return nil, err
	}
	out := []model.UsageRecord{}
	for _, item := range items {
		if !validAmount(item.GetCost()) {
			log.Printf("modal: skipping %q: unparseable cost %q", item.GetObjectId(), item.GetCost())
			continue
		}
		if item.GetInterval() == nil {
			log.Printf("modal: skipping %q: missing interval", item.GetObjectId())
			continue
		}
		out = append(out, s.toRecord(item))
	}

	summary, err := s.summarize(ctx, start)
	if err != nil {
		log.Printf("modal: billing summary unavailable, skipping non-usage charges: %v", err)
		return out, nil
	}
	out = append(out, s.adjustmentRecords(summary)...)
	return out, nil
}

func (s *source) adjustmentRecords(summary *modalpb.WorkspaceBillingSummaryResponse) []model.UsageRecord {
	periodStart := summary.GetStartTimestamp().AsTime().UTC()
	periodEnd := summary.GetEndTimestamp().AsTime().UTC()
	var out []model.UsageRecord
	for key, amount := range summary.GetAdjustments() {
		if amount == "" {
			continue
		}
		out = append(out, s.adjustmentRecord(key, amount, periodStart, periodEnd))
	}
	return out
}

func (s *source) adjustmentRecord(key, amount string, periodStart, periodEnd time.Time) model.UsageRecord {
	cost := model.Dec(amount)
	category, pricing, label := adjustmentClass(key)
	rec := model.UsageRecord{
		Provider:          "Modal",
		Publisher:         "Modal",
		InvoiceIssuer:     "Modal",
		ServiceName:       "Modal",
		ServiceCategory:   model.ServiceCategoryCompute,
		ChargeCategory:    category,
		ChargeFrequency:   model.ChargeFrequencyRecurring,
		PricingCategory:   pricing,
		Currency:          "USD",
		PricingCurrency:   "USD",
		BillingAccountID:  s.accountID,
		ChargeDescription: label,
		SkuMeter:          key,
		Cost:              &cost,
		PeriodStart:       &periodStart,
		PeriodEnd:         &periodEnd,
	}
	return rec
}

func adjustmentClass(key string) (model.ChargeCategory, model.PricingCategory, string) {
	switch key {
	case "plan":
		return model.ChargePurchase, model.PricingStandard, "Modal plan fee"
	case "reservations":
		return model.ChargePurchase, model.PricingCommitted, "Modal reservation"
	case "credits":
		return model.ChargeCredit, model.PricingStandard, "Modal credits"
	case "storage":
		return model.ChargeUsage, model.PricingStandard, "Modal storage"
	default:
		return model.ChargeAdjustment, model.PricingStandard, "Modal " + key
	}
}

func (s *source) toRecord(item *modalpb.WorkspaceBillingReportItem) model.UsageRecord {
	cost := model.Dec(item.GetCost())
	rec := model.UsageRecord{
		Provider:           "Modal",
		Publisher:          "Modal",
		InvoiceIssuer:      "Modal",
		ServiceName:        "Modal",
		ServiceCategory:    model.ServiceCategoryCompute,
		ServiceSubcategory: model.ServiceSubcategoryServerlessCompute,
		ChargeCategory:     model.ChargeUsage,
		ChargeFrequency:    model.ChargeFrequencyUsageBased,
		PricingCategory:    model.PricingStandard,
		Currency:           "USD",
		PricingCurrency:    "USD",
		BillingAccountID:   s.accountID,
		Day:                item.GetInterval().AsTime().UTC(),
		Cost:               &cost,
		ResourceID:         item.GetObjectId(),
		ResourceName:       item.GetDescription(),
		ResourceType:       objectType(item.GetObjectId()),
		SkuID:              item.GetObjectId(),
		ChargeDescription:  chargeDescription(item),
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

func dialModal() (modalpb.ModalClientClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return nil, nil, fmt.Errorf("modal: dial: %w", err)
	}
	return modalpb.NewModalClientClient(conn), conn, nil
}

func authContext(ctx context.Context, tokenID, tokenSecret string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
		"x-modal-token-id":     tokenID,
		"x-modal-token-secret": tokenSecret,
	}))
}

func grpcReporter(tokenID, tokenSecret string) reporter {
	return func(ctx context.Context, start, end time.Time) (items []*modalpb.WorkspaceBillingReportItem, err error) {
		client, conn, err := dialModal()
		if err != nil {
			return nil, err
		}
		defer func() {
			if cerr := conn.Close(); cerr != nil {
				err = errors.Join(err, fmt.Errorf("modal: close: %w", cerr))
			}
		}()

		ctx, cancel := context.WithTimeout(authContext(ctx, tokenID, tokenSecret), callTimeout)
		defer cancel()

		req := &modalpb.WorkspaceBillingReportRequest{
			StartTimestamp: timestamppb.New(start.UTC()),
			EndTimestamp:   timestamppb.New(end.UTC()),
			Resolution:     "d",
		}
		stream, err := client.WorkspaceBillingReport(ctx, req)
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

func grpcSummarizer(tokenID, tokenSecret string) summarizer {
	return func(ctx context.Context, start time.Time) (summary *modalpb.WorkspaceBillingSummaryResponse, err error) {
		client, conn, err := dialModal()
		if err != nil {
			return nil, err
		}
		defer func() {
			if cerr := conn.Close(); cerr != nil {
				err = errors.Join(err, fmt.Errorf("modal: close: %w", cerr))
			}
		}()

		ctx, cancel := context.WithTimeout(authContext(ctx, tokenID, tokenSecret), callTimeout)
		defer cancel()

		req := &modalpb.WorkspaceBillingSummaryRequest{StartTimestamp: timestamppb.New(start.UTC())}
		summary, err = client.WorkspaceBillingSummary(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("modal: WorkspaceBillingSummary: %w", err)
		}
		return summary, nil
	}
}

var Provider = integrations.Provider{
	Name:         Name,
	Capabilities: integrations.Capabilities{RequiresTimeRange: true},
	New: func(get integrations.HTTPGet, env func(string) string) (integrations.Source, error) {
		tokenID := env("MODAL_TOKEN_ID")
		tokenSecret := env("MODAL_TOKEN_SECRET")
		workspace := env("MODAL_WORKSPACE_ID")
		if tokenID == "" || tokenSecret == "" || workspace == "" {
			return nil, fmt.Errorf("missing MODAL_TOKEN_ID / MODAL_TOKEN_SECRET / MODAL_WORKSPACE_ID env")
		}
		return New(get, tokenID, tokenSecret, workspace), nil
	},
}
