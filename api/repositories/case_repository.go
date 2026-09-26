package repositories

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

type participantDoc struct {
	UserID   string    `bson:"userId"`
	Name     string    `bson:"name"`
	Role     string    `bson:"role"`
	JoinedAt time.Time `bson:"joinedAt"`
}

type caseDoc struct {
	ID           string           `bson:"_id"`
	Subject      string           `bson:"subject"`
	CustomerID   string           `bson:"customerId"`
	Participants []participantDoc `bson:"participants"`
	Status       string           `bson:"status"`
	CreatedAt    time.Time        `bson:"createdAt"`
	UpdatedAt    time.Time        `bson:"updatedAt"`
	ClosedAt     *time.Time       `bson:"closedAt,omitempty"`
	Version      int              `bson:"version"`
}

type MongoCaseRepository struct {
	col *mongo.Collection
}

var _ usecases.CaseRepository = (*MongoCaseRepository)(nil)

func NewMongoCaseRepository(db *mongo.Database) *MongoCaseRepository {
	return &MongoCaseRepository{col: db.Collection(casesCollection)}
}

func (r *MongoCaseRepository) Insert(ctx context.Context, c models.Case) error {
	_, err := r.col.InsertOne(ctx, toCaseDoc(c))
	return err
}

func (r *MongoCaseRepository) Get(ctx context.Context, id string) (models.Case, error) {
	var d caseDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return models.Case{}, models.ErrNotFound
	}
	if err != nil {
		return models.Case{}, err
	}
	return d.toModel(), nil
}

func (r *MongoCaseRepository) Update(ctx context.Context, c models.Case) error {
	d := toCaseDoc(c)
	d.Version = c.Version + 1
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": c.ID, "version": c.Version}, d)
	if err != nil {
		return err
	}
	if res.MatchedCount == 1 {
		return nil
	}
	n, err := r.col.CountDocuments(ctx, bson.M{"_id": c.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return models.ErrConflict
}

func (r *MongoCaseRepository) List(ctx context.Context, f usecases.CaseFilter) ([]models.Case, error) {
	filter := bson.M{}
	if f.CustomerID != "" {
		filter["customerId"] = f.CustomerID
	}
	if f.Status != "" {
		filter["status"] = string(f.Status)
	}
	cur, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var docs []caseDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]models.Case, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toModel())
	}
	return out, nil
}

func toCaseDoc(c models.Case) caseDoc {
	ps := make([]participantDoc, 0, len(c.Participants))
	for _, p := range c.Participants {
		ps = append(ps, participantDoc{UserID: p.UserID, Name: p.Name, Role: string(p.Role), JoinedAt: p.JoinedAt})
	}
	return caseDoc{
		ID:           c.ID,
		Subject:      c.Subject,
		CustomerID:   c.CustomerID,
		Participants: ps,
		Status:       string(c.Status),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
		ClosedAt:     c.ClosedAt,
		Version:      c.Version,
	}
}

func (d caseDoc) toModel() models.Case {
	ps := make([]models.Participant, 0, len(d.Participants))
	for _, p := range d.Participants {
		ps = append(ps, models.Participant{UserID: p.UserID, Name: p.Name, Role: models.Role(p.Role), JoinedAt: p.JoinedAt.UTC()})
	}
	var closedAt *time.Time
	if d.ClosedAt != nil {
		t := d.ClosedAt.UTC()
		closedAt = &t
	}
	return models.Case{
		ID:           d.ID,
		Subject:      d.Subject,
		CustomerID:   d.CustomerID,
		Participants: ps,
		Status:       models.CaseStatus(d.Status),
		CreatedAt:    d.CreatedAt.UTC(),
		UpdatedAt:    d.UpdatedAt.UTC(),
		ClosedAt:     closedAt,
		Version:      d.Version,
	}
}
