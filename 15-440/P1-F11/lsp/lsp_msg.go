package lsp

func genConnMsg(id uint16) *LspMsg {
	return &LspMsg{MsgCONNECT, id, 0, []byte("MsgCONNECT")}
}

func genAckMsg(id uint16, seqnum byte) *LspMsg {
	return &LspMsg{MsgACK, id, seqnum, []byte("MsgACK")}
}

func genDataMsg(id uint16, seqnum byte, data []byte) *LspMsg {
	return &LspMsg{MsgDATA, id, seqnum, data}
}
