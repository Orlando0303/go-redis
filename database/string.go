package database

import (
	"go-redis/interface/database"
	"go-redis/interface/resp"
	"go-redis/lib/utils"
	"go-redis/resp/reply"
	"strconv"
	"strings"
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

const (
	upsertPolicy = iota // default
	insertPolicy        // set nx
	updatePolicy        // set ex
)

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
	policy := upsertPolicy
	// parse options
	if len(args) > 2 {
		for i := 2; i < len(args); i++ {
			arg := strings.ToUpper(string(args[i]))
			if arg == "NX" { // insert
				if policy == updatePolicy {
					return &reply.SyntaxErrReply{}
				}
				policy = insertPolicy
			} else if arg == "XX" { // update policy
				if policy == insertPolicy {
					return &reply.SyntaxErrReply{}
				}
				policy = updatePolicy
			} else {
				return &reply.SyntaxErrReply{}
			}
		}
	}

	entity := &database.DataEntity{
		Data: value,
	}

	var result int
	switch policy {
	case upsertPolicy:
		db.PutEntity(key, entity)
		result = 1
	case insertPolicy:
		result = db.PutIfAbsent(key, entity)
	case updatePolicy:
		result = db.PutIfExists(key, entity)
	}
	db.addAof(utils.ToCmdLine2("set", args...))
	if result > 0 {
		return &reply.OKReply{}
	}
	return &reply.NullBulkReply{}
}

// execSetNX sets string if not exists
func execSetNX(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]
	entity := &database.DataEntity{
		Data: value,
	}
	result := db.PutIfAbsent(key, entity)
	db.addAof(utils.ToCmdLine2("setnx", args...))
	return reply.MakeIntReply(int64(result))
}

// execMSet sets multi key-value in database
func execMSet(db *DB, args [][]byte) resp.Reply {
	if len(args)%2 != 0 {
		return reply.MakeSyntaxErrReply()
	}

	size := len(args) / 2
	keys := make([]string, size)
	values := make([][]byte, size)
	for i := 0; i < size; i++ {
		keys[i] = string(args[2*i])
		values[i] = args[2*i+1]
	}

	for i, key := range keys {
		value := values[i]
		db.PutEntity(key, &database.DataEntity{Data: value})
	}
	db.addAof(utils.ToCmdLine2("mset", args...))
	return &reply.OKReply{}
}

// execMGet get multi key-value from database
func execMGet(db *DB, args [][]byte) resp.Reply {
	keys := make([]string, len(args))
	for i, v := range args {
		keys[i] = string(v)
	}

	result := make([][]byte, len(args))
	for i, key := range keys {
		bytes, err := db.getAsString(key)
		if err != nil {
			_, isWrongType := err.(*reply.WrongTypeErrReply)
			if isWrongType {
				result[i] = nil
				continue
			} else {
				return err
			}
		}
		result[i] = bytes // nil or []byte
	}

	return reply.MakeMultiBulkReply(result)
}

// execMSetNX sets multi key-value in database, only if none of the given keys exist
func execMSetNX(db *DB, args [][]byte) resp.Reply {
	// parse args
	if len(args)%2 != 0 {
		return reply.MakeSyntaxErrReply()
	}
	size := len(args) / 2
	values := make([][]byte, size)
	keys := make([]string, size)
	for i := 0; i < size; i++ {
		keys[i] = string(args[2*i])
		values[i] = args[2*i+1]
	}

	for _, key := range keys {
		_, exists := db.GetEntity(key)
		if exists {
			return reply.MakeIntReply(0)
		}
	}

	for i, key := range keys {
		value := values[i]
		db.PutEntity(key, &database.DataEntity{Data: value})
	}
	db.addAof(utils.ToCmdLine2("msetnx", args...))
	return reply.MakeIntReply(1)
}

// execGetSet sets value of a string-type key and returns its old value
func execGetSet(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	value := args[1]

	old, err := db.getAsString(key)
	if err != nil {
		return err
	}
	db.PutEntity(key, &database.DataEntity{Data: value})
	if old == nil {
		return new(reply.NullBulkReply)
	}
	db.addAof(utils.ToCmdLine2("getset", args...))
	return reply.MakeBulkReply(old)
}

// execIncr increments the integer value of a key by one
func execIncr(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	bytes, err := db.getAsString(key)
	if err != nil {
		return err
	}
	if bytes != nil {
		val, err := strconv.ParseInt(string(bytes), 10, 64)
		if err != nil {
			return reply.MakeErrReply("ERR value is not an integer or out of range")
		}
		db.PutEntity(key, &database.DataEntity{
			Data: []byte(strconv.FormatInt(val+1, 10)),
		})
		db.addAof(utils.ToCmdLine2("incr", args...))
		return reply.MakeIntReply(val + 1)
	}
	db.PutEntity(key, &database.DataEntity{
		Data: []byte("1"),
	})
	db.addAof(utils.ToCmdLine2("incr", args...))
	return reply.MakeIntReply(1)
}

// execIncrBy increments the integer value of a key by given value
func execIncrBy(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	rawDelta := string(args[1])
	delta, err := strconv.ParseInt(rawDelta, 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}

	bytes, errReply := db.getAsString(key)
	if errReply != nil {
		return errReply
	}
	if bytes != nil {
		// existed value
		val, err := strconv.ParseInt(string(bytes), 10, 64)
		if err != nil {
			return reply.MakeErrReply("ERR value is not an integer or out of range")
		}
		db.PutEntity(key, &database.DataEntity{
			Data: []byte(strconv.FormatInt(val+delta, 10)),
		})
		db.addAof(utils.ToCmdLine2("incrby", args...))
		return reply.MakeIntReply(val + delta)
	}
	db.PutEntity(key, &database.DataEntity{
		Data: args[1],
	})
	db.addAof(utils.ToCmdLine2("incrby", args...))
	return reply.MakeIntReply(delta)
}

// execDecr decrements the integer value of a key by one
func execDecr(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])

	bytes, errReply := db.getAsString(key)
	if errReply != nil {
		return errReply
	}
	if bytes != nil {
		val, err := strconv.ParseInt(string(bytes), 10, 64)
		if err != nil {
			return reply.MakeErrReply("ERR value is not an integer or out of range")
		}
		db.PutEntity(key, &database.DataEntity{
			Data: []byte(strconv.FormatInt(val-1, 10)),
		})
		db.addAof(utils.ToCmdLine2("decr", args...))
		return reply.MakeIntReply(val - 1)
	}
	entity := &database.DataEntity{
		Data: []byte("-1"),
	}
	db.PutEntity(key, entity)
	db.addAof(utils.ToCmdLine2("decr", args...))
	return reply.MakeIntReply(-1)
}

// execDecrBy decrements the integer value of a key by onedecrement
func execDecrBy(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	rawDelta := string(args[1])
	delta, err := strconv.ParseInt(rawDelta, 10, 64)
	if err != nil {
		return reply.MakeErrReply("ERR value is not an integer or out of range")
	}

	bytes, errReply := db.getAsString(key)
	if errReply != nil {
		return errReply
	}
	if bytes != nil {
		val, err := strconv.ParseInt(string(bytes), 10, 64)
		if err != nil {
			return reply.MakeErrReply("ERR value is not an integer or out of range")
		}
		db.PutEntity(key, &database.DataEntity{
			Data: []byte(strconv.FormatInt(val-delta, 10)),
		})
		db.addAof(utils.ToCmdLine2("decrby", args...))
		return reply.MakeIntReply(val - delta)
	}
	valueStr := strconv.FormatInt(-delta, 10)
	db.PutEntity(key, &database.DataEntity{
		Data: []byte(valueStr),
	})
	db.addAof(utils.ToCmdLine2("decrby", args...))
	return reply.MakeIntReply(-delta)
}

// execStrLen returns len of string value bound to the given key
func execStrLen(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	bytes, err := db.getAsString(key)
	if err != nil {
		return err
	}
	if bytes == nil {
		return reply.MakeIntReply(0)
	}
	return reply.MakeIntReply(int64(len(bytes)))
}

// execAppend sets string value to the given key
func execAppend(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	bytes, err := db.getAsString(key)
	if err != nil {
		return err
	}
	bytes = append(bytes, args[1]...)
	db.PutEntity(key, &database.DataEntity{
		Data: bytes,
	})
	db.addAof(utils.ToCmdLine2("append", args...))
	return reply.MakeIntReply(int64(len(bytes)))
}

// execSetRange overwrites part of the string stored at key, starting at the specified offset.
// If the offset is larger than the current length of the string at key, the string is padded with zero-bytes.
func execSetRange(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	offset, errNative := strconv.ParseInt(string(args[1]), 10, 64)
	if errNative != nil {
		return reply.MakeErrReply(errNative.Error())
	}
	value := args[2]
	bytes, err := db.getAsString(key)
	if err != nil {
		return err
	}
	bytesLen := int64(len(bytes))
	if bytesLen < offset {
		diff := offset - bytesLen
		diffArray := make([]byte, diff)
		bytes = append(bytes, diffArray...)
		bytesLen = int64(len(bytes))
	}
	for i := 0; i < len(value); i++ {
		idx := offset + int64(i)
		if idx >= bytesLen {
			bytes = append(bytes, value[i])
		} else {
			bytes[idx] = value[i]
		}
	}
	db.PutEntity(key, &database.DataEntity{
		Data: bytes,
	})
	db.addAof(utils.ToCmdLine2("setRange", args...))
	return reply.MakeIntReply(int64(len(bytes)))
}

func execGetRange(db *DB, args [][]byte) resp.Reply {
	key := string(args[0])
	startIdx, errNative := strconv.ParseInt(string(args[1]), 10, 64)
	if errNative != nil {
		return reply.MakeErrReply(errNative.Error())
	}
	endIdx, errNative := strconv.ParseInt(string(args[2]), 10, 64)
	if errNative != nil {
		return reply.MakeErrReply(errNative.Error())
	}

	bytes, err := db.getAsString(key)
	if err != nil {
		return err
	}

	if bytes == nil {
		return reply.MakeNullBulkReply()
	}

	bytesLen := int64(len(bytes))
	if startIdx < -1*bytesLen {
		return &reply.NullBulkReply{}
	} else if startIdx < 0 {
		startIdx = bytesLen + startIdx
	} else if startIdx >= bytesLen {
		return &reply.NullBulkReply{}
	}
	if endIdx < -1*bytesLen {
		return &reply.NullBulkReply{}
	} else if endIdx < 0 {
		endIdx = bytesLen + endIdx + 1
	} else if endIdx < bytesLen {
		endIdx = endIdx + 1
	} else {
		endIdx = bytesLen
	}
	if startIdx > endIdx {
		return reply.MakeNullBulkReply()
	}

	return reply.MakeBulkReply(bytes[startIdx:endIdx])
}

func init() {
	RegisterCommand("Set", execSet, -3)
	RegisterCommand("SetNx", execSetNX, 3)
	RegisterCommand("MSet", execMSet, -3)
	RegisterCommand("MGet", execMGet, -2)
	RegisterCommand("MSetNX", execMSetNX, -3)
	RegisterCommand("Get", execGet, 2)
	RegisterCommand("GetSet", execGetSet, 3)
	RegisterCommand("Incr", execIncr, 2)
	RegisterCommand("IncrBy", execIncrBy, 3)
	RegisterCommand("Decr", execDecr, 2)
	RegisterCommand("DecrBy", execDecrBy, 3)
	RegisterCommand("StrLen", execStrLen, 2)
	RegisterCommand("Append", execAppend, 3)
	RegisterCommand("SetRange", execSetRange, 4)
	RegisterCommand("GetRange", execGetRange, 4)
}
