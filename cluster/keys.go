package cluster

import (
	"go-redis/interface/resp"
	"go-redis/resp/reply"
)

// FlushDB removes all data in current database
func FlushDB(cluster *ClusterDatabase, c resp.Connection, args [][]byte) resp.Reply {
	replies := cluster.broadcast(c, args)
	var errReply resp.ErrorReply
	for _, v := range replies {
		if reply.IsErrorReply(v) {
			errReply = v.(resp.ErrorReply)
			break
		}
	}
	if errReply == nil {
		return &reply.OKReply{}
	}
	return reply.MakeErrReply("error occurs: " + errReply.Error())
}
