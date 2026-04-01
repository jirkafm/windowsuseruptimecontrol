package helperipc

type CommandType string

const (
	CommandSpeak CommandType = "speak"
)

type Command struct {
	Type    CommandType `json:"type"`
	Message string      `json:"message"`
}
