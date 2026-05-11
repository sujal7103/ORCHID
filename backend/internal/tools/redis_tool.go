package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisReadTool creates a tool for reading from Redis
func NewRedisReadTool() *Tool {
	return &Tool{
		Name:        "redis_read",
		DisplayName: "Redis Read",
		Description: "Read data from Redis. Supports GET, MGET, HGET, HGETALL, LRANGE, SMEMBERS, ZRANGE, KEYS, EXISTS, TTL, and TYPE operations.",
		Icon:        "Database",
		Source:      ToolSourceBuiltin,
		Category:    "database",
		Keywords:    []string{"redis", "cache", "database", "key-value", "read", "get", "memory"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"get", "mget", "hget", "hgetall", "lrange", "smembers", "zrange", "keys", "exists", "ttl", "type"},
					"description": "The read operation to perform",
				},
				"key": map[string]interface{}{
					"type":        "string",
					"description": "The Redis key to read",
				},
				"keys": map[string]interface{}{
					"type":        "array",
					"description": "Array of keys (for MGET operation)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"field": map[string]interface{}{
					"type":        "string",
					"description": "Hash field name (for HGET operation)",
				},
				"start": map[string]interface{}{
					"type":        "integer",
					"description": "Start index for LRANGE/ZRANGE (default: 0)",
				},
				"stop": map[string]interface{}{
					"type":        "integer",
					"description": "Stop index for LRANGE/ZRANGE (default: -1 for all)",
				},
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Pattern for KEYS operation (e.g., 'user:*'). Use with caution on large databases.",
				},
			},
			"required": []string{"action"},
		},
		Execute: executeRedisRead,
	}
}

// NewRedisWriteTool creates a tool for writing to Redis (no delete operations)
func NewRedisWriteTool() *Tool {
	return &Tool{
		Name:        "redis_write",
		DisplayName: "Redis Write",
		Description: "Write data to Redis. Supports SET, MSET, HSET, LPUSH, RPUSH, SADD, ZADD, INCR, and EXPIRE operations. Delete operations are not permitted for safety.",
		Icon:        "DatabaseBackup",
		Source:      ToolSourceBuiltin,
		Category:    "database",
		Keywords:    []string{"redis", "cache", "database", "key-value", "write", "set", "memory", "store"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"set", "mset", "hset", "lpush", "rpush", "sadd", "zadd", "incr", "incrby", "expire", "setnx", "setex"},
					"description": "The write operation to perform (delete is not permitted)",
				},
				"key": map[string]interface{}{
					"type":        "string",
					"description": "The Redis key",
				},
				"value": map[string]interface{}{
					"description": "The value to set (for SET, SETEX, SETNX operations). Can be string, number, or object (will be JSON encoded).",
				},
				"values": map[string]interface{}{
					"type":        "object",
					"description": "Key-value pairs for MSET operation",
				},
				"field": map[string]interface{}{
					"type":        "string",
					"description": "Hash field name (for HSET operation)",
				},
				"field_values": map[string]interface{}{
					"type":        "object",
					"description": "Multiple field-value pairs for HSET (alternative to single field/value)",
				},
				"members": map[string]interface{}{
					"type":        "array",
					"description": "Array of members for LPUSH, RPUSH, SADD operations",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"scored_members": map[string]interface{}{
					"type":        "array",
					"description": "Array of {score, member} objects for ZADD operation",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"score": map[string]interface{}{
								"type": "number",
							},
							"member": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				"ttl_seconds": map[string]interface{}{
					"type":        "integer",
					"description": "Time-to-live in seconds (for SETEX, EXPIRE operations)",
				},
				"increment": map[string]interface{}{
					"type":        "integer",
					"description": "Increment value (for INCRBY operation)",
				},
			},
			"required": []string{"action"},
		},
		Execute: executeRedisWrite,
	}
}

func getRedisClient(args map[string]interface{}) (*redis.Client, error) {
	// Get credential data
	credData, err := GetCredentialData(args, "redis")
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis credentials: %w", err)
	}

	host, _ := credData["host"].(string)
	if host == "" {
		host = "localhost"
	}

	port, _ := credData["port"].(string)
	if port == "" {
		// Try as number
		if portNum, ok := credData["port"].(float64); ok {
			port = fmt.Sprintf("%.0f", portNum)
		} else {
			port = "6379"
		}
	}

	password, _ := credData["password"].(string)

	db := 0
	if dbNum, ok := credData["database"].(float64); ok {
		db = int(dbNum)
	} else if dbNum, ok := credData["db"].(float64); ok {
		db = int(dbNum)
	}

	// Check for connection_string (alternative format)
	if connStr, ok := credData["connection_string"].(string); ok && connStr != "" {
		opt, err := redis.ParseURL(connStr)
		if err != nil {
			return nil, fmt.Errorf("invalid Redis connection string: %w", err)
		}
		return redis.NewClient(opt), nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	return client, nil
}

func executeRedisRead(args map[string]interface{}) (string, error) {
	client, err := getRedisClient(args)
	if err != nil {
		return "", err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return "", fmt.Errorf("failed to connect to Redis: %w", err)
	}

	action, _ := args["action"].(string)
	key, _ := args["key"].(string)

	var result interface{}

	switch action {
	case "get":
		if key == "" {
			return "", fmt.Errorf("key is required for GET operation")
		}
		val, err := client.Get(ctx, key).Result()
		if err == redis.Nil {
			result = map[string]interface{}{
				"exists": false,
				"value":  nil,
			}
		} else if err != nil {
			return "", fmt.Errorf("GET failed: %w", err)
		} else {
			// Try to parse as JSON
			var jsonVal interface{}
			if json.Unmarshal([]byte(val), &jsonVal) == nil {
				result = map[string]interface{}{
					"exists": true,
					"value":  jsonVal,
				}
			} else {
				result = map[string]interface{}{
					"exists": true,
					"value":  val,
				}
			}
		}

	case "mget":
		keys, ok := args["keys"].([]interface{})
		if !ok || len(keys) == 0 {
			return "", fmt.Errorf("keys array is required for MGET operation")
		}
		strKeys := make([]string, len(keys))
		for i, k := range keys {
			strKeys[i] = fmt.Sprintf("%v", k)
		}

		vals, err := client.MGet(ctx, strKeys...).Result()
		if err != nil {
			return "", fmt.Errorf("MGET failed: %w", err)
		}

		resultMap := make(map[string]interface{})
		for i, k := range strKeys {
			if vals[i] != nil {
				resultMap[k] = vals[i]
			} else {
				resultMap[k] = nil
			}
		}
		result = resultMap

	case "hget":
		if key == "" {
			return "", fmt.Errorf("key is required for HGET operation")
		}
		field, _ := args["field"].(string)
		if field == "" {
			return "", fmt.Errorf("field is required for HGET operation")
		}
		val, err := client.HGet(ctx, key, field).Result()
		if err == redis.Nil {
			result = map[string]interface{}{
				"exists": false,
				"value":  nil,
			}
		} else if err != nil {
			return "", fmt.Errorf("HGET failed: %w", err)
		} else {
			result = map[string]interface{}{
				"exists": true,
				"value":  val,
			}
		}

	case "hgetall":
		if key == "" {
			return "", fmt.Errorf("key is required for HGETALL operation")
		}
		val, err := client.HGetAll(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("HGETALL failed: %w", err)
		}
		result = map[string]interface{}{
			"exists": len(val) > 0,
			"fields": val,
		}

	case "lrange":
		if key == "" {
			return "", fmt.Errorf("key is required for LRANGE operation")
		}
		start := int64(0)
		stop := int64(-1)
		if s, ok := args["start"].(float64); ok {
			start = int64(s)
		}
		if s, ok := args["stop"].(float64); ok {
			stop = int64(s)
		}
		vals, err := client.LRange(ctx, key, start, stop).Result()
		if err != nil {
			return "", fmt.Errorf("LRANGE failed: %w", err)
		}
		result = map[string]interface{}{
			"count":    len(vals),
			"elements": vals,
		}

	case "smembers":
		if key == "" {
			return "", fmt.Errorf("key is required for SMEMBERS operation")
		}
		vals, err := client.SMembers(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("SMEMBERS failed: %w", err)
		}
		result = map[string]interface{}{
			"count":   len(vals),
			"members": vals,
		}

	case "zrange":
		if key == "" {
			return "", fmt.Errorf("key is required for ZRANGE operation")
		}
		start := int64(0)
		stop := int64(-1)
		if s, ok := args["start"].(float64); ok {
			start = int64(s)
		}
		if s, ok := args["stop"].(float64); ok {
			stop = int64(s)
		}
		vals, err := client.ZRangeWithScores(ctx, key, start, stop).Result()
		if err != nil {
			return "", fmt.Errorf("ZRANGE failed: %w", err)
		}
		members := make([]map[string]interface{}, len(vals))
		for i, z := range vals {
			members[i] = map[string]interface{}{
				"member": z.Member,
				"score":  z.Score,
			}
		}
		result = map[string]interface{}{
			"count":   len(vals),
			"members": members,
		}

	case "keys":
		pattern, _ := args["pattern"].(string)
		if pattern == "" {
			return "", fmt.Errorf("pattern is required for KEYS operation")
		}
		// Use SCAN instead of KEYS for safety on large databases
		var cursor uint64
		var allKeys []string
		for {
			keys, nextCursor, err := client.Scan(ctx, cursor, pattern, 100).Result()
			if err != nil {
				return "", fmt.Errorf("SCAN failed: %w", err)
			}
			allKeys = append(allKeys, keys...)
			cursor = nextCursor
			if cursor == 0 {
				break
			}
			// Limit to 1000 keys for safety
			if len(allKeys) >= 1000 {
				allKeys = allKeys[:1000]
				break
			}
		}
		result = map[string]interface{}{
			"count": len(allKeys),
			"keys":  allKeys,
		}

	case "exists":
		if key == "" {
			return "", fmt.Errorf("key is required for EXISTS operation")
		}
		exists, err := client.Exists(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("EXISTS failed: %w", err)
		}
		result = map[string]interface{}{
			"exists": exists > 0,
		}

	case "ttl":
		if key == "" {
			return "", fmt.Errorf("key is required for TTL operation")
		}
		ttl, err := client.TTL(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("TTL failed: %w", err)
		}
		result = map[string]interface{}{
			"ttl_seconds":   int64(ttl.Seconds()),
			"has_expiry":    ttl > 0,
			"no_expiry":     ttl == -1,
			"key_not_found": ttl == -2,
		}

	case "type":
		if key == "" {
			return "", fmt.Errorf("key is required for TYPE operation")
		}
		keyType, err := client.Type(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("TYPE failed: %w", err)
		}
		result = map[string]interface{}{
			"type": keyType,
		}

	default:
		return "", fmt.Errorf("unsupported read action: %s", action)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

func executeRedisWrite(args map[string]interface{}) (string, error) {
	client, err := getRedisClient(args)
	if err != nil {
		return "", err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return "", fmt.Errorf("failed to connect to Redis: %w", err)
	}

	action, _ := args["action"].(string)
	key, _ := args["key"].(string)

	var result interface{}

	switch action {
	case "set":
		if key == "" {
			return "", fmt.Errorf("key is required for SET operation")
		}
		value := args["value"]
		if value == nil {
			return "", fmt.Errorf("value is required for SET operation")
		}
		// Serialize complex values to JSON
		var strVal string
		switch v := value.(type) {
		case string:
			strVal = v
		default:
			jsonBytes, _ := json.Marshal(v)
			strVal = string(jsonBytes)
		}

		err := client.Set(ctx, key, strVal, 0).Err()
		if err != nil {
			return "", fmt.Errorf("SET failed: %w", err)
		}
		result = map[string]interface{}{
			"success": true,
			"key":     key,
		}

	case "setex":
		if key == "" {
			return "", fmt.Errorf("key is required for SETEX operation")
		}
		value := args["value"]
		if value == nil {
			return "", fmt.Errorf("value is required for SETEX operation")
		}
		ttl, ok := args["ttl_seconds"].(float64)
		if !ok || ttl <= 0 {
			return "", fmt.Errorf("positive ttl_seconds is required for SETEX operation")
		}

		var strVal string
		switch v := value.(type) {
		case string:
			strVal = v
		default:
			jsonBytes, _ := json.Marshal(v)
			strVal = string(jsonBytes)
		}

		err := client.SetEx(ctx, key, strVal, time.Duration(ttl)*time.Second).Err()
		if err != nil {
			return "", fmt.Errorf("SETEX failed: %w", err)
		}
		result = map[string]interface{}{
			"success":     true,
			"key":         key,
			"ttl_seconds": int64(ttl),
		}

	case "setnx":
		if key == "" {
			return "", fmt.Errorf("key is required for SETNX operation")
		}
		value := args["value"]
		if value == nil {
			return "", fmt.Errorf("value is required for SETNX operation")
		}

		var strVal string
		switch v := value.(type) {
		case string:
			strVal = v
		default:
			jsonBytes, _ := json.Marshal(v)
			strVal = string(jsonBytes)
		}

		wasSet, err := client.SetNX(ctx, key, strVal, 0).Result()
		if err != nil {
			return "", fmt.Errorf("SETNX failed: %w", err)
		}
		result = map[string]interface{}{
			"success": true,
			"was_set": wasSet,
			"key":     key,
		}

	case "mset":
		values, ok := args["values"].(map[string]interface{})
		if !ok || len(values) == 0 {
			return "", fmt.Errorf("values object is required for MSET operation")
		}

		pairs := make([]interface{}, 0, len(values)*2)
		for k, v := range values {
			var strVal string
			switch val := v.(type) {
			case string:
				strVal = val
			default:
				jsonBytes, _ := json.Marshal(val)
				strVal = string(jsonBytes)
			}
			pairs = append(pairs, k, strVal)
		}

		err := client.MSet(ctx, pairs...).Err()
		if err != nil {
			return "", fmt.Errorf("MSET failed: %w", err)
		}
		result = map[string]interface{}{
			"success": true,
			"count":   len(values),
		}

	case "hset":
		if key == "" {
			return "", fmt.Errorf("key is required for HSET operation")
		}

		// Support both single field/value and multiple field_values
		fieldValues := make(map[string]interface{})
		if fv, ok := args["field_values"].(map[string]interface{}); ok {
			fieldValues = fv
		} else if field, ok := args["field"].(string); ok && field != "" {
			fieldValues[field] = args["value"]
		} else {
			return "", fmt.Errorf("field/value or field_values is required for HSET operation")
		}

		values := make([]interface{}, 0, len(fieldValues)*2)
		for k, v := range fieldValues {
			var strVal string
			switch val := v.(type) {
			case string:
				strVal = val
			default:
				jsonBytes, _ := json.Marshal(val)
				strVal = string(jsonBytes)
			}
			values = append(values, k, strVal)
		}

		count, err := client.HSet(ctx, key, values...).Result()
		if err != nil {
			return "", fmt.Errorf("HSET failed: %w", err)
		}
		result = map[string]interface{}{
			"success":     true,
			"key":         key,
			"fields_set":  count,
		}

	case "lpush":
		if key == "" {
			return "", fmt.Errorf("key is required for LPUSH operation")
		}
		members, ok := args["members"].([]interface{})
		if !ok || len(members) == 0 {
			return "", fmt.Errorf("members array is required for LPUSH operation")
		}

		count, err := client.LPush(ctx, key, members...).Result()
		if err != nil {
			return "", fmt.Errorf("LPUSH failed: %w", err)
		}
		result = map[string]interface{}{
			"success":     true,
			"key":         key,
			"list_length": count,
		}

	case "rpush":
		if key == "" {
			return "", fmt.Errorf("key is required for RPUSH operation")
		}
		members, ok := args["members"].([]interface{})
		if !ok || len(members) == 0 {
			return "", fmt.Errorf("members array is required for RPUSH operation")
		}

		count, err := client.RPush(ctx, key, members...).Result()
		if err != nil {
			return "", fmt.Errorf("RPUSH failed: %w", err)
		}
		result = map[string]interface{}{
			"success":     true,
			"key":         key,
			"list_length": count,
		}

	case "sadd":
		if key == "" {
			return "", fmt.Errorf("key is required for SADD operation")
		}
		members, ok := args["members"].([]interface{})
		if !ok || len(members) == 0 {
			return "", fmt.Errorf("members array is required for SADD operation")
		}

		count, err := client.SAdd(ctx, key, members...).Result()
		if err != nil {
			return "", fmt.Errorf("SADD failed: %w", err)
		}
		result = map[string]interface{}{
			"success":     true,
			"key":         key,
			"added_count": count,
		}

	case "zadd":
		if key == "" {
			return "", fmt.Errorf("key is required for ZADD operation")
		}
		scoredMembers, ok := args["scored_members"].([]interface{})
		if !ok || len(scoredMembers) == 0 {
			return "", fmt.Errorf("scored_members array is required for ZADD operation")
		}

		zMembers := make([]redis.Z, 0, len(scoredMembers))
		for _, sm := range scoredMembers {
			if m, ok := sm.(map[string]interface{}); ok {
				score, _ := m["score"].(float64)
				member := fmt.Sprintf("%v", m["member"])
				zMembers = append(zMembers, redis.Z{Score: score, Member: member})
			}
		}

		count, err := client.ZAdd(ctx, key, zMembers...).Result()
		if err != nil {
			return "", fmt.Errorf("ZADD failed: %w", err)
		}
		result = map[string]interface{}{
			"success":     true,
			"key":         key,
			"added_count": count,
		}

	case "incr":
		if key == "" {
			return "", fmt.Errorf("key is required for INCR operation")
		}
		newVal, err := client.Incr(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("INCR failed: %w", err)
		}
		result = map[string]interface{}{
			"success":   true,
			"key":       key,
			"new_value": newVal,
		}

	case "incrby":
		if key == "" {
			return "", fmt.Errorf("key is required for INCRBY operation")
		}
		increment, ok := args["increment"].(float64)
		if !ok {
			return "", fmt.Errorf("increment is required for INCRBY operation")
		}
		newVal, err := client.IncrBy(ctx, key, int64(increment)).Result()
		if err != nil {
			return "", fmt.Errorf("INCRBY failed: %w", err)
		}
		result = map[string]interface{}{
			"success":   true,
			"key":       key,
			"new_value": newVal,
		}

	case "expire":
		if key == "" {
			return "", fmt.Errorf("key is required for EXPIRE operation")
		}
		ttl, ok := args["ttl_seconds"].(float64)
		if !ok || ttl <= 0 {
			return "", fmt.Errorf("positive ttl_seconds is required for EXPIRE operation")
		}

		wasSet, err := client.Expire(ctx, key, time.Duration(ttl)*time.Second).Result()
		if err != nil {
			return "", fmt.Errorf("EXPIRE failed: %w", err)
		}
		result = map[string]interface{}{
			"success":     true,
			"key":         key,
			"was_set":     wasSet,
			"ttl_seconds": int64(ttl),
		}

	case "del", "delete":
		if key == "" {
			return "", fmt.Errorf("key is required for DEL operation")
		}

		deleted, err := client.Del(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("DEL failed: %w", err)
		}
		result = map[string]interface{}{
			"success":       true,
			"key":           key,
			"deleted_count": deleted,
		}

	default:
		return "", fmt.Errorf("unsupported write action: %s", action)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}
