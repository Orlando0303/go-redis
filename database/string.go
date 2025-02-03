package database

import (
	"go-redis/interface/database"
	"go-redis/interface/resp"
	"go-redis/resp/reply"
)

// 把val断言为string
func (db *DB) getAsString(key string) ([]byte, resp.Reply) {
	entity, ok := db.GetEntity(key)
	if !ok {
		return nil, reply.MakeNullBulkReply()
	}
	bytes, ok := entity.Data.([]byte)
	if !ok {
		return nil, &reply.WrongTypeErrReply{}
	}
	return bytes, nil
}

// execGet returns string value bound to the given key
func execGet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	bytes, err := db.getAsString(key)
	if err != nil {
		return err
	}
	if bytes == nil {
		return &reply.NullBulkReply{}
	}
	return reply.MakeBulkReply(bytes)
}

// execSet sets string value and time to live to the given key
func execSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity := &database.DataEntity{
		Data: value,
	}
	db.PutEntity(key, entity)
	return reply.MakeOKReply()
}

// execSetNX sets string if not exists
func execSetNX(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity := &database.DataEntity{
		Data: value,
	}
	result := db.PutIfAbsent(key, entity)
	return reply.MakeIntReply(int64(result))
}

// execGetSet sets value of a string-type key and returns its old value
func execGetSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]

	old, err := db.getAsString(key)
	db.PutEntity(key, &database.DataEntity{Data: value})
	if err != nil {
		return err
	}
	if old == nil {
		return &reply.NullBulkReply{}
	}

	return reply.MakeBulkReply(old)
}

// execStrLen returns len of string value bound to the given key
func execStrLen(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	old, err := db.getAsString(key)
	if err != nil {
		return err
	}
	if old == nil {
		return &reply.NullBulkReply{}
	}
	return reply.MakeIntReply(int64(len(old)))
}

func init() {
	RegisterCommand("Get", execGet, 2)
	RegisterCommand("Set", execSet, 3)
	RegisterCommand("SetNx", execSetNX, 3)
	RegisterCommand("GetSet", execGetSet, 3)
	RegisterCommand("StrLen", execStrLen, 2)
}
