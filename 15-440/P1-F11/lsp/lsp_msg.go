package lsp

func genAckMsg(id uint16, seqnum byte) *LspMsg {
	return &LspMsg{MsgACK, id, seqnum, nil}
}

func genDataMsg(id uint16, seqnum byte, data []byte) *LspMsg {
	return &LspMsg{MsgDATA, id, seqnum, data}
}
