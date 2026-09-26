package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

type messageDoc struct {
	ID         string    `bson:"_id"`
	CaseID     string    `bson:"caseId"`
	SenderID   string    `bson:"senderId"`
	SenderName string    `bson:"senderName"`
	SenderRole string    `bson:"senderRole"`
	Kind       string    `bson:"kind"`
	Body       string    `bson:"body"`
	CreatedAt  time.Time `bson:"createdAt"`
}

type MongoMessageRepository struct {
	col *mongo.Collection
}

var _ usecases.MessageRepository = (*MongoMessageRepository)(nil)

func NewMongoMessageRepository(db *mongo.Database) *MongoMessageRepository {
	return &MongoMessageRepository{col: db.Collection(messagesCollection)}
}

func (r *MongoMessageRepository) Insert(ctx context.Context, m models.Message) error {
	_, err := r.col.InsertOne(ctx, messageDoc{
		ID:         m.ID,
		CaseID:     m.CaseID,
		SenderID:   m.SenderID,
		SenderName: m.SenderName,
		SenderRole: string(m.SenderRole),
		Kind:       string(m.Kind),
		Body:       m.Body,
		CreatedAt:  m.CreatedAt,
	})
	return err
}

// List pages on _id rather than createdAt: many messages can share one millisecond,
// but IDs are unique and sort in creation order (UUIDv7).
func (r *MongoMessageRepository) List(ctx context.Context, q usecases.MessageQuery) ([]models.Message, error) {
	filter := bson.M{"caseId": q.CaseID}
	if q.BeforeID != "" {
		filter["_id"] = bson.M{"$lt": q.BeforeID}
	}
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(q.Limit))
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var docs []messageDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]models.Message, 0, len(docs))
	for _, d := range docs {
		out = append(out, models.Message{
			ID:         d.ID,
			CaseID:     d.CaseID,
			SenderID:   d.SenderID,
			SenderName: d.SenderName,
			SenderRole: models.Role(d.SenderRole),
			Kind:       models.MessageKind(d.Kind),
			Body:       d.Body,
			CreatedAt:  d.CreatedAt.UTC(),
		})
	}
	return out, nil
}
