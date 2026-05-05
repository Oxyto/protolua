package backend

func uniquePtr(id string, data any) bsonDocument {
	return bsonDocument{
		"ID":   id,
		"Data": data,
	}
}

func worldElementRef(refID, targetID string) bsonDocument {
	return uniquePtr(refID, targetID)
}

func nullWorldElementRef(refID string) bsonDocument {
	return worldElementRef(refID, "")
}

func syncValue(value any) any {
	return anyToBSON(value)
}

func syncList(id string, values bsonArray) bsonDocument {
	return uniquePtr(id, values)
}
