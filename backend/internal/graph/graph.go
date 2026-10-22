// Package graph exposes the store and AI pipeline as a GraphQL API.
package graph

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/Hrishabh003/creator-commerce-platform/backend/internal/ai"
	"github.com/Hrishabh003/creator-commerce-platform/backend/internal/store"
	"github.com/graphql-go/graphql"
)

func nn(t graphql.Output) graphql.Output { return graphql.NewNonNull(t) }

func idArg(p graphql.ResolveParams, name string) (int64, error) {
	s, _ := p.Args[name].(string)
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errors.New("invalid " + name)
	}
	return id, nil
}

func strArg(p graphql.ResolveParams, name string) string {
	s, _ := p.Args[name].(string)
	return s
}

func limitArg(p graphql.ResolveParams) int {
	if n, ok := p.Args["limit"].(int); ok && n > 0 && n <= 200 {
		return n
	}
	return 50
}

func NewSchema(st *store.Store, pipe *ai.Pipeline) (graphql.Schema, error) {
	customer := graphql.NewObject(graphql.ObjectConfig{Name: "Customer", Fields: graphql.Fields{
		"id": {Type: nn(graphql.ID)}, "name": {Type: nn(graphql.String)}, "email": {Type: nn(graphql.String)},
	}})
	product := graphql.NewObject(graphql.ObjectConfig{Name: "Product", Fields: graphql.Fields{
		"id": {Type: nn(graphql.ID)}, "sku": {Type: nn(graphql.String)}, "name": {Type: nn(graphql.String)},
		"priceCents": {Type: nn(graphql.Int)}, "stock": {Type: nn(graphql.Int)},
	}})
	order := graphql.NewObject(graphql.ObjectConfig{Name: "Order", Fields: graphql.Fields{
		"id": {Type: nn(graphql.ID)}, "status": {Type: nn(graphql.String)},
		"totalCents": {Type: nn(graphql.Int)}, "createdAt": {Type: nn(graphql.DateTime)},
		"customer": {Type: customer},
	}})
	draft := graphql.NewObject(graphql.ObjectConfig{Name: "Draft", Fields: graphql.Fields{
		"id": {Type: nn(graphql.ID)}, "ticketId": {Type: nn(graphql.ID)}, "body": {Type: nn(graphql.String)},
		"status":    {Type: nn(graphql.String)},
		"sources":   {Type: nn(graphql.NewList(nn(graphql.String)))},
		"createdAt": {Type: nn(graphql.DateTime)},
	}})
	ticket := graphql.NewObject(graphql.ObjectConfig{Name: "Ticket", Fields: graphql.Fields{
		"id": {Type: nn(graphql.ID)}, "subject": {Type: nn(graphql.String)}, "body": {Type: nn(graphql.String)},
		"category": {Type: nn(graphql.String)}, "status": {Type: nn(graphql.String)},
		"createdAt": {Type: nn(graphql.DateTime)}, "customer": {Type: customer},
		"drafts": {Type: nn(graphql.NewList(nn(draft)))},
	}})
	campaign := graphql.NewObject(graphql.ObjectConfig{Name: "Campaign", Fields: graphql.Fields{
		"id": {Type: nn(graphql.ID)}, "name": {Type: nn(graphql.String)}, "subject": {Type: nn(graphql.String)},
		"status": {Type: nn(graphql.String)}, "sentCount": {Type: nn(graphql.Int)},
		"scheduledFor": {Type: graphql.DateTime},
	}})

	listArgs := graphql.FieldConfigArgument{
		"status": {Type: graphql.String},
		"limit":  {Type: graphql.Int},
	}

	query := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{
		"products": {Type: nn(graphql.NewList(nn(product))),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) { return st.Products(p.Context) }},
		"orders": {Type: nn(graphql.NewList(nn(order))), Args: listArgs,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return st.Orders(p.Context, strArg(p, "status"), limitArg(p))
			}},
		"tickets": {Type: nn(graphql.NewList(nn(ticket))), Args: listArgs,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return st.Tickets(p.Context, strArg(p, "status"), limitArg(p))
			}},
		"ticket": {Type: ticket, Args: graphql.FieldConfigArgument{"id": {Type: nn(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				id, err := idArg(p, "id")
				if err != nil {
					return nil, err
				}
				return st.Ticket(p.Context, id)
			}},
		"campaigns": {Type: nn(graphql.NewList(nn(campaign))),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) { return st.Campaigns(p.Context) }},
	}})

	mutation := graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: graphql.Fields{
		// createTicket stores the ticket, then runs categorize -> retrieve -> draft.
		// The draft is saved as "pending"; nothing is sent until an operator approves it.
		"createTicket": {Type: nn(ticket), Args: graphql.FieldConfigArgument{
			"customerId": {Type: nn(graphql.ID)},
			"subject":    {Type: nn(graphql.String)},
			"body":       {Type: nn(graphql.String)},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			cid, err := idArg(p, "customerId")
			if err != nil {
				return nil, err
			}
			subject, body := strArg(p, "subject"), strArg(p, "body")
			if subject == "" || body == "" {
				return nil, errors.New("subject and body are required")
			}
			customers, err := st.CustomersByID(p.Context, []int64{cid})
			if err != nil {
				return nil, err
			}
			c, ok := customers[cid]
			if !ok {
				return nil, errors.New("customer not found")
			}
			res, perr := pipe.Process(p.Context, c.Name, subject, body)
			category := res.Category
			if category == "" {
				category = "general"
			}
			tid, err := st.CreateTicket(p.Context, cid, subject, body, category)
			if err != nil {
				return nil, err
			}
			if perr != nil {
				log.Printf("ai pipeline failed for ticket %d: %v", tid, perr)
			} else if err := st.CreateDraft(p.Context, tid, res.Draft, res.Sources); err != nil {
				return nil, err
			}
			return st.Ticket(p.Context, tid)
		}},
		"reviewDraft": {Type: nn(draft), Args: graphql.FieldConfigArgument{
			"id":      {Type: nn(graphql.ID)},
			"approve": {Type: nn(graphql.Boolean)},
			"body":    {Type: graphql.String},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			id, err := idArg(p, "id")
			if err != nil {
				return nil, err
			}
			approve, _ := p.Args["approve"].(bool)
			var body *string
			if s, ok := p.Args["body"].(string); ok {
				body = &s
			}
			return st.ReviewDraft(p.Context, id, approve, body)
		}},
		"updateStock": {Type: nn(product), Args: graphql.FieldConfigArgument{
			"productId": {Type: nn(graphql.ID)},
			"stock":     {Type: nn(graphql.Int)},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			id, err := idArg(p, "productId")
			if err != nil {
				return nil, err
			}
			stock, _ := p.Args["stock"].(int)
			if stock < 0 {
				return nil, errors.New("stock cannot be negative")
			}
			return st.UpdateStock(p.Context, id, stock)
		}},
	}})

	return graphql.NewSchema(graphql.SchemaConfig{Query: query, Mutation: mutation})
}

// Handler serves POST /graphql with {query, variables, operationName}.
func Handler(schema graphql.Schema) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Query         string                 `json:"query"`
			Variables     map[string]interface{} `json:"variables"`
			OperationName string                 `json:"operationName"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		res := graphql.Do(graphql.Params{
			Schema: schema, RequestString: req.Query, VariableValues: req.Variables,
			OperationName: req.OperationName, Context: r.Context(),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})
}
