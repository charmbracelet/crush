package message

type Attachment struct {
	FilePath string
	FileName string
	MimeType string
	Content  []byte
}
