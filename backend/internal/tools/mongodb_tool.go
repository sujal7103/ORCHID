package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewMongoDBQueryTool creates a tool for querying MongoDB (read operations)
func NewMongoDBQueryTool() *Tool {
	return &Tool{
		Name:        "mongodb_query",
		DisplayName: "MongoDB Query",
		Description: "Query documents from a MongoDB collection. Supports find, findOne, count, and aggregate operations. Use this for reading data from MongoDB.",
		Icon:        "Database",
		Source:      ToolSourceBuiltin,
		Category:    "database",
		Keywords:    []string{"mongodb", "mongo", "database", "query", "find", "read", "nosql", "document"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"find", "findOne", "count", "aggregate"},
					"description": "The query operation to perform",
				},
				"collection": map[string]interface{}{
					"type":        "string",
					"description": "The MongoDB collection name to query",
				},
				"filter": map[string]interface{}{
					"type":        "object",
					"description": "MongoDB filter/query object (e.g., {\"status\": \"active\"})",
				},
				"projection": map[string]interface{}{
					"type":        "object",
					"description": "Fields to include/exclude (e.g., {\"name\": 1, \"email\": 1})",
				},
				"sort": map[string]interface{}{
					"type":        "object",
					"description": "Sort order (e.g., {\"createdAt\": -1} for descending)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of documents to return (default: 100, max: 1000)",
				},
				"skip": map[string]interface{}{
					"type":        "integer",
					"description": "Number of documents to skip (for pagination)",
				},
				"pipeline": map[string]interface{}{
					"type":        "array",
					"description": "Aggregation pipeline stages (only for aggregate action)",
					"items": map[string]interface{}{
						"type": "object",
					},
				},
			},
			"required": []string{"action", "collection"},
		},
		Execute: executeMongoDBQuery,
	}
}

// NewMongoDBWriteTool creates a tool for writing to MongoDB (insert/update operations only, no delete)
func NewMongoDBWriteTool() *Tool {
	return &Tool{
		Name:        "mongodb_write",
		DisplayName: "MongoDB Write",
		Description: "Insert or update documents in a MongoDB collection. IMPORTANT: You must use the exact action names: 'insertOne' (single doc), 'insertMany' (multiple docs), 'updateOne', or 'updateMany'. Do NOT use generic names like 'insert' or 'update'. Delete operations are not permitted for safety.",
		Icon:        "DatabaseBackup",
		Source:      ToolSourceBuiltin,
		Category:    "database",
		Keywords:    []string{"mongodb", "mongo", "database", "insert", "update", "write", "nosql", "document", "create"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"insertOne", "insertMany", "updateOne", "updateMany"},
					"description": "REQUIRED: Must be one of: 'insertOne' (insert single document), 'insertMany' (insert multiple documents), 'updateOne' (update single document), 'updateMany' (update multiple documents). Do NOT use 'insert' or 'update' - use the specific variant.",
				},
				"collection": map[string]interface{}{
					"type":        "string",
					"description": "The MongoDB collection name",
				},
				"document": map[string]interface{}{
					"type":        "object",
					"description": "Document to insert (for insertOne)",
				},
				"documents": map[string]interface{}{
					"type":        "array",
					"description": "Array of documents to insert (for insertMany)",
					"items": map[string]interface{}{
						"type": "object",
					},
				},
				"filter": map[string]interface{}{
					"type":        "object",
					"description": "Filter to match documents for update operations",
				},
				"update": map[string]interface{}{
					"type":        "object",
					"description": "Update operations (e.g., {\"$set\": {\"status\": \"completed\"}})",
				},
				"upsert": map[string]interface{}{
					"type":        "boolean",
					"description": "Create document if it doesn't exist (for update operations)",
				},
			},
			"required": []string{"action", "collection"},
		},
		Execute: executeMongoDBWrite,
	}
}

func executeMongoDBQuery(args map[string]interface{}) (string, error) {
	// Get credential data
	credData, err := GetCredentialData(args, "mongodb")
	if err != nil {
		return "", fmt.Errorf("failed to get MongoDB credentials: %w", err)
	}

	connectionString, _ := credData["connection_string"].(string)
	if connectionString == "" {
		return "", fmt.Errorf("connection_string is required in credentials")
	}

	databaseName, _ := credData["database"].(string)
	if databaseName == "" {
		return "", fmt.Errorf("database name is required in credentials")
	}

	// Get action parameters
	action, _ := args["action"].(string)
	collectionName, _ := args["collection"].(string)

	if action == "" {
		return "", fmt.Errorf("action is required")
	}
	if collectionName == "" {
		return "", fmt.Errorf("collection is required")
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionString))
	if err != nil {
		return "", fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return "", fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	collection := client.Database(databaseName).Collection(collectionName)

	// Parse filter
	filter := bson.M{}
	if f, ok := args["filter"].(map[string]interface{}); ok {
		filter = convertToBsonM(f)
	}

	var result interface{}

	switch action {
	case "findOne":
		var doc bson.M
		opts := options.FindOne()
		if proj, ok := args["projection"].(map[string]interface{}); ok {
			opts.SetProjection(convertToBsonM(proj))
		}
		err = collection.FindOne(ctx, filter, opts).Decode(&doc)
		if err == mongo.ErrNoDocuments {
			result = map[string]interface{}{
				"found":    false,
				"document": nil,
			}
		} else if err != nil {
			return "", fmt.Errorf("findOne failed: %w", err)
		} else {
			result = map[string]interface{}{
				"found":    true,
				"document": convertBsonToMap(doc),
			}
		}

	case "find":
		opts := options.Find()
		if proj, ok := args["projection"].(map[string]interface{}); ok {
			opts.SetProjection(convertToBsonM(proj))
		}
		if sort, ok := args["sort"].(map[string]interface{}); ok {
			opts.SetSort(convertToBsonM(sort))
		}
		limit := int64(100) // Default limit
		if l, ok := args["limit"].(float64); ok {
			limit = int64(l)
			if limit > 1000 {
				limit = 1000 // Max limit for safety
			}
		}
		opts.SetLimit(limit)
		if skip, ok := args["skip"].(float64); ok {
			opts.SetSkip(int64(skip))
		}

		cursor, err := collection.Find(ctx, filter, opts)
		if err != nil {
			return "", fmt.Errorf("find failed: %w", err)
		}
		defer cursor.Close(ctx)

		var docs []bson.M
		if err := cursor.All(ctx, &docs); err != nil {
			return "", fmt.Errorf("failed to decode results: %w", err)
		}

		convertedDocs := make([]map[string]interface{}, len(docs))
		for i, doc := range docs {
			convertedDocs[i] = convertBsonToMap(doc)
		}

		result = map[string]interface{}{
			"count":     len(docs),
			"documents": convertedDocs,
		}

	case "count":
		count, err := collection.CountDocuments(ctx, filter)
		if err != nil {
			return "", fmt.Errorf("count failed: %w", err)
		}
		result = map[string]interface{}{
			"count": count,
		}

	case "aggregate":
		pipeline, ok := args["pipeline"].([]interface{})
		if !ok || len(pipeline) == 0 {
			return "", fmt.Errorf("pipeline is required for aggregate action")
		}

		// Convert pipeline stages
		mongoPipeline := make([]bson.M, len(pipeline))
		for i, stage := range pipeline {
			if stageMap, ok := stage.(map[string]interface{}); ok {
				mongoPipeline[i] = convertToBsonM(stageMap)
			}
		}

		cursor, err := collection.Aggregate(ctx, mongoPipeline)
		if err != nil {
			return "", fmt.Errorf("aggregate failed: %w", err)
		}
		defer cursor.Close(ctx)

		var docs []bson.M
		if err := cursor.All(ctx, &docs); err != nil {
			return "", fmt.Errorf("failed to decode aggregate results: %w", err)
		}

		convertedDocs := make([]map[string]interface{}, len(docs))
		for i, doc := range docs {
			convertedDocs[i] = convertBsonToMap(doc)
		}

		result = map[string]interface{}{
			"count":   len(docs),
			"results": convertedDocs,
		}

	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

func executeMongoDBWrite(args map[string]interface{}) (string, error) {
	// Get credential data
	credData, err := GetCredentialData(args, "mongodb")
	if err != nil {
		return "", fmt.Errorf("failed to get MongoDB credentials: %w", err)
	}

	connectionString, _ := credData["connection_string"].(string)
	if connectionString == "" {
		return "", fmt.Errorf("connection_string is required in credentials")
	}

	databaseName, _ := credData["database"].(string)
	if databaseName == "" {
		return "", fmt.Errorf("database name is required in credentials")
	}

	// Get action parameters
	action, _ := args["action"].(string)
	collectionName, _ := args["collection"].(string)

	if action == "" {
		return "", fmt.Errorf("action is required")
	}
	if collectionName == "" {
		return "", fmt.Errorf("collection is required")
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionString))
	if err != nil {
		return "", fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return "", fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	collection := client.Database(databaseName).Collection(collectionName)

	var result interface{}

	switch action {
	case "insertOne":
		doc, ok := args["document"].(map[string]interface{})
		if !ok || len(doc) == 0 {
			return "", fmt.Errorf("document is required for insertOne")
		}

		insertResult, err := collection.InsertOne(ctx, convertToBsonM(doc))
		if err != nil {
			return "", fmt.Errorf("insertOne failed: %w", err)
		}

		result = map[string]interface{}{
			"success":     true,
			"inserted_id": formatObjectID(insertResult.InsertedID),
		}

	case "insertMany":
		docs, ok := args["documents"].([]interface{})
		if !ok || len(docs) == 0 {
			return "", fmt.Errorf("documents array is required for insertMany")
		}

		// Limit batch size for safety
		if len(docs) > 1000 {
			return "", fmt.Errorf("maximum 1000 documents can be inserted at once")
		}

		bsonDocs := make([]interface{}, len(docs))
		for i, doc := range docs {
			if docMap, ok := doc.(map[string]interface{}); ok {
				bsonDocs[i] = convertToBsonM(docMap)
			}
		}

		insertResult, err := collection.InsertMany(ctx, bsonDocs)
		if err != nil {
			return "", fmt.Errorf("insertMany failed: %w", err)
		}

		insertedIDs := make([]string, len(insertResult.InsertedIDs))
		for i, id := range insertResult.InsertedIDs {
			insertedIDs[i] = formatObjectID(id)
		}

		result = map[string]interface{}{
			"success":       true,
			"inserted_count": len(insertResult.InsertedIDs),
			"inserted_ids":  insertedIDs,
		}

	case "updateOne":
		filter, ok := args["filter"].(map[string]interface{})
		if !ok || len(filter) == 0 {
			return "", fmt.Errorf("filter is required for updateOne")
		}

		update, ok := args["update"].(map[string]interface{})
		if !ok || len(update) == 0 {
			return "", fmt.Errorf("update is required for updateOne")
		}

		opts := options.Update()
		if upsert, ok := args["upsert"].(bool); ok {
			opts.SetUpsert(upsert)
		}

		updateResult, err := collection.UpdateOne(ctx, convertToBsonM(filter), convertToBsonM(update), opts)
		if err != nil {
			return "", fmt.Errorf("updateOne failed: %w", err)
		}

		result = map[string]interface{}{
			"success":        true,
			"matched_count":  updateResult.MatchedCount,
			"modified_count": updateResult.ModifiedCount,
			"upserted_id":    formatObjectID(updateResult.UpsertedID),
		}

	case "updateMany":
		filter, ok := args["filter"].(map[string]interface{})
		if !ok || len(filter) == 0 {
			return "", fmt.Errorf("filter is required for updateMany")
		}

		update, ok := args["update"].(map[string]interface{})
		if !ok || len(update) == 0 {
			return "", fmt.Errorf("update is required for updateMany")
		}

		opts := options.Update()
		if upsert, ok := args["upsert"].(bool); ok {
			opts.SetUpsert(upsert)
		}

		updateResult, err := collection.UpdateMany(ctx, convertToBsonM(filter), convertToBsonM(update), opts)
		if err != nil {
			return "", fmt.Errorf("updateMany failed: %w", err)
		}

		result = map[string]interface{}{
			"success":        true,
			"matched_count":  updateResult.MatchedCount,
			"modified_count": updateResult.ModifiedCount,
			"upserted_id":    formatObjectID(updateResult.UpsertedID),
		}

	// Handle common LLM mistakes - suggest the correct action
	case "insert":
		return "", fmt.Errorf("action 'insert' is not valid. Did you mean 'insertOne' (for single document) or 'insertMany' (for multiple documents)?")
	case "update":
		return "", fmt.Errorf("action 'update' is not valid. Did you mean 'updateOne' (for single document) or 'updateMany' (for multiple documents)?")
	default:
		return "", fmt.Errorf("unsupported action: %s (only insertOne, insertMany, updateOne, updateMany are permitted)", action)
	}

	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonResult), nil
}

// convertToBsonM converts a map[string]interface{} to bson.M
func convertToBsonM(m map[string]interface{}) bson.M {
	result := bson.M{}
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = convertToBsonM(val)
		case []interface{}:
			result[k] = convertToBsonArray(val)
		default:
			result[k] = v
		}
	}
	return result
}

// convertToBsonArray converts []interface{} to bson.A
func convertToBsonArray(arr []interface{}) bson.A {
	result := make(bson.A, len(arr))
	for i, v := range arr {
		switch val := v.(type) {
		case map[string]interface{}:
			result[i] = convertToBsonM(val)
		case []interface{}:
			result[i] = convertToBsonArray(val)
		default:
			result[i] = v
		}
	}
	return result
}

// convertBsonToMap converts bson.M to map[string]interface{} for JSON serialization
func convertBsonToMap(m bson.M) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case bson.M:
			result[k] = convertBsonToMap(val)
		case primitive.ObjectID:
			result[k] = val.Hex()
		case primitive.DateTime:
			result[k] = val.Time().Format(time.RFC3339)
		case bson.A:
			result[k] = convertBsonArrayToSlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

// convertBsonArrayToSlice converts bson.A to []interface{} for JSON serialization
func convertBsonArrayToSlice(arr bson.A) []interface{} {
	result := make([]interface{}, len(arr))
	for i, v := range arr {
		switch val := v.(type) {
		case bson.M:
			result[i] = convertBsonToMap(val)
		case primitive.ObjectID:
			result[i] = val.Hex()
		case primitive.DateTime:
			result[i] = val.Time().Format(time.RFC3339)
		case bson.A:
			result[i] = convertBsonArrayToSlice(val)
		default:
			result[i] = v
		}
	}
	return result
}

// formatObjectID formats an ObjectID for JSON output
func formatObjectID(id interface{}) string {
	if id == nil {
		return ""
	}
	if oid, ok := id.(primitive.ObjectID); ok {
		return oid.Hex()
	}
	return fmt.Sprintf("%v", id)
}
