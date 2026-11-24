package notification

type Data struct {
	ID        int    `json:"-"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	RelatedId int    `json:"relatedId"`
	IsRead    bool   `json:"isRead"`
	CreatedAt int64  `json:"createdAt"`
}
