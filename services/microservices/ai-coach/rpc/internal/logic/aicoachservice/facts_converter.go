package aicoachservicelogic

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
)

// protoUserFact maps a stored fact to its wire form.
//
// superseded_by / superseded_at are deliberately NOT exposed: every fact these
// RPCs return is a current one, and surfacing the supersession chain to clients
// would invite them to render retired beliefs as if the coach still held them.
func protoUserFact(f db.UserFact) *aicoach.UserFact {
	var createdAt int64
	if f.CreatedAt.Valid {
		createdAt = f.CreatedAt.Time.Unix()
	}
	return &aicoach.UserFact{
		Id:           f.ID.String(),
		Fact:         f.Fact,
		Category:     f.Category,
		Confidence:   f.Confidence,
		UserAuthored: f.UserAuthored,
		CreatedAt:    createdAt,
	}
}
