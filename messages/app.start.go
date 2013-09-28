package messages

type AppStart struct {
	Guid  string `json:"guid"`
	Index int    `json:"index"`
}
