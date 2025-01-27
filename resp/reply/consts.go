package reply

// pong reply
type PongReply struct{}

var pongBytes = []byte("+PONG\r\n")

func (r *PongReply) ToBytes() []byte {
	return pongBytes
}

var thePongReply = new(PongReply)

func MakePongReply() *PongReply {
	return thePongReply
}

// OK reply
type OKReply struct{}

var okBytes = []byte("+OK\r\n")

func (r *OKReply) ToBytes() []byte {
	return okBytes
}

var theOKReply = new(OKReply)

func MakeOKReply() *OKReply {
	return theOKReply
}

// empty string reply
type NullBulkReply struct{}

var nullBulkBytes = []byte("$-1\r\n")

func (r *NullBulkReply) ToBytes() []byte {
	return nullBulkBytes
}

var theNullBulkReply = new(NullBulkReply)

func MakeNullBulkReply() *NullBulkReply {
	return theNullBulkReply
}

// empty list reply
type EmptyMultiBulkReply struct{}

var emptyMultiBulkBytes = []byte("*0\r\n")

func (r *EmptyMultiBulkReply) ToBytes() []byte {
	return emptyMultiBulkBytes
}

var theEmptyMultiBulkReply = new(EmptyMultiBulkReply)

func MakeEmptyMultiBulkReply() *EmptyMultiBulkReply {
	return theEmptyMultiBulkReply
}

// nothing reply
type NoReply struct{}

var noBytes = []byte("")

func (r *NoReply) ToBytes() []byte {
	return noBytes
}

var theNoReply = new(NoReply)

func MakeNoReply() *NoReply {
	return theNoReply
}
