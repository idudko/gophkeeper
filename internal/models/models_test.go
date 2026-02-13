// Package models provides tests for data models.
package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataType_Values(t *testing.T) {
	assert.Equal(t, "password", string(DataTypePassword))
	assert.Equal(t, "text", string(DataTypeText))
	assert.Equal(t, "binary", string(DataTypeBinary))
	assert.Equal(t, "card", string(DataTypeCard))
}

func TestUser_MarshalJSON(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:             uuid.New().String(),
		Email:          "test@example.com",
		Password:       "hashed-password",
		EncryptionSalt: "salt123",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	data, err := json.Marshal(user)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Password and encryption salt should not be in JSON
	_, hasPassword := result["password"]
	assert.False(t, hasPassword, "password should not be marshaled to JSON")

	_, hasSalt := result["encryption_salt"]
	assert.False(t, hasSalt, "encryption_salt should not be marshaled to JSON")

	// Check other fields
	assert.Equal(t, user.ID, result["id"])
	assert.Equal(t, user.Email, result["email"])
}

func TestUser_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "550e8400-e29b-41d4-a716-446655440000",
		"email": "test@example.com",
		"created_at": "2024-01-01T00:00:00Z",
		"updated_at": "2024-01-01T00:00:00Z"
	}`

	var user User
	err := json.Unmarshal([]byte(jsonData), &user)
	require.NoError(t, err)

	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestPasswordData_MarshalUnmarshal(t *testing.T) {
	data := &PasswordData{
		Meta:     "GitHub",
		Login:    "user123",
		Password: "encrypted-password",
	}

	jsonBytes, err := json.Marshal(data)
	require.NoError(t, err)

	var result PasswordData
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	assert.Equal(t, data.Meta, result.Meta)
	assert.Equal(t, data.Login, result.Login)
	assert.Equal(t, data.Password, result.Password)
}

func TestTextData_MarshalUnmarshal(t *testing.T) {
	data := &TextData{
		Meta: "Important note",
		Data: "This is a secret note",
	}

	jsonBytes, err := json.Marshal(data)
	require.NoError(t, err)

	var result TextData
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	assert.Equal(t, data.Meta, result.Meta)
	assert.Equal(t, data.Data, result.Data)
}

func TestBinaryData_MarshalUnmarshal(t *testing.T) {
	data := &BinaryData{
		Meta:     "Secret file",
		Filename: "secret.txt",
		Data:     []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x10},
	}

	jsonBytes, err := json.Marshal(data)
	require.NoError(t, err)

	var result BinaryData
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	assert.Equal(t, data.Meta, result.Meta)
	assert.Equal(t, data.Filename, result.Filename)
	assert.Equal(t, data.Data, result.Data)
}

func TestCardData_MarshalUnmarshal(t *testing.T) {
	data := &CardData{
		Meta:       "Personal card",
		Number:     "4111111111111111",
		HolderName: "John Doe",
		ExpiryDate: "12/25",
		CVV:        "123",
	}

	jsonBytes, err := json.Marshal(data)
	require.NoError(t, err)

	var result CardData
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)

	assert.Equal(t, data.Meta, result.Meta)
	assert.Equal(t, data.Number, result.Number)
	assert.Equal(t, data.HolderName, result.HolderName)
	assert.Equal(t, data.ExpiryDate, result.ExpiryDate)
	assert.Equal(t, data.CVV, result.CVV)
}

func TestItem_PasswordType(t *testing.T) {
	item := &Item{
		ID:        uuid.New().String(),
		UserID:    uuid.New().String(),
		Type:      DataTypePassword,
		Version:   1,
		Meta:      "GitHub",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Password: &PasswordData{
			Meta:     "GitHub",
			Login:    "user123",
			Password: "encrypted-password",
		},
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var result Item
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Password)
	assert.Equal(t, item.Password.Meta, result.Password.Meta)
	assert.Equal(t, item.Password.Login, result.Password.Login)
	assert.Equal(t, item.Password.Password, result.Password.Password)
}

func TestItem_TextType(t *testing.T) {
	item := &Item{
		ID:        uuid.New().String(),
		UserID:    uuid.New().String(),
		Type:      DataTypeText,
		Version:   1,
		Meta:      "Note",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Text: &TextData{
			Meta: "Important",
			Data: "This is a note",
		},
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var result Item
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Text)
	assert.Equal(t, item.Text.Meta, result.Text.Meta)
	assert.Equal(t, item.Text.Data, result.Text.Data)
}

func TestItem_BinaryType(t *testing.T) {
	item := &Item{
		ID:        uuid.New().String(),
		UserID:    uuid.New().String(),
		Type:      DataTypeBinary,
		Version:   1,
		Meta:      "File",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Binary: &BinaryData{
			Meta:     "Document",
			Filename: "doc.pdf",
			Data:     []byte("PDF content"),
		},
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var result Item
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Binary)
	assert.Equal(t, item.Binary.Meta, result.Binary.Meta)
	assert.Equal(t, item.Binary.Filename, result.Binary.Filename)
	assert.Equal(t, item.Binary.Data, result.Binary.Data)
}

func TestItem_CardType(t *testing.T) {
	item := &Item{
		ID:        uuid.New().String(),
		UserID:    uuid.New().String(),
		Type:      DataTypeCard,
		Version:   1,
		Meta:      "Bank card",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Card: &CardData{
			Meta:       "Personal",
			Number:     "4111111111111111",
			HolderName: "John Doe",
			ExpiryDate: "12/25",
			CVV:        "123",
		},
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var result Item
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.NotNil(t, result.Card)
	assert.Equal(t, item.Card.Meta, result.Card.Meta)
	assert.Equal(t, item.Card.Number, result.Card.Number)
	assert.Equal(t, item.Card.HolderName, result.Card.HolderName)
	assert.Equal(t, item.Card.ExpiryDate, result.Card.ExpiryDate)
	assert.Equal(t, item.Card.CVV, result.Card.CVV)
}

func TestItem_DeletedAt(t *testing.T) {
	now := time.Now()
	item := &Item{
		ID:        uuid.New().String(),
		UserID:    uuid.New().String(),
		Type:      DataTypeText,
		Version:   1,
		CreatedAt: now.Truncate(time.Millisecond),
		UpdatedAt: now.Truncate(time.Millisecond),
	}

	t.Run("not deleted", func(t *testing.T) {
		data, err := json.Marshal(item)
		require.NoError(t, err)

		var result Item
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Nil(t, result.DeletedAt)
	})

	t.Run("deleted", func(t *testing.T) {
		deletedTime := now.Add(1 * time.Hour).Truncate(time.Millisecond)
		item.DeletedAt = &deletedTime

		data, err := json.Marshal(item)
		require.NoError(t, err)

		var result Item
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.NotNil(t, result.DeletedAt)
		// Compare truncated times to handle JSON precision
		assert.Equal(t, deletedTime.Truncate(time.Millisecond), result.DeletedAt.Truncate(time.Millisecond))
	})
}

func TestItem_VersionIncrement(t *testing.T) {
	item := &Item{
		ID:        uuid.New().String(),
		UserID:    uuid.New().String(),
		Type:      DataTypeText,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Text: &TextData{
			Data: "Version 1",
		},
	}

	// Increment version
	item.Version++

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var result Item
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, item.ID, result.ID)
	assert.Equal(t, item.UserID, result.UserID)
	assert.Equal(t, item.Type, result.Type)
	assert.Equal(t, 2, result.Version)
}

func TestSyncRequest_MarshalUnmarshal(t *testing.T) {
	items := []Item{
		{
			ID:      uuid.New().String(),
			Type:    DataTypePassword,
			Version: 1,
			Password: &PasswordData{
				Login:    "user1",
				Password: "pass1",
			},
		},
		{
			ID:      uuid.New().String(),
			Type:    DataTypeText,
			Version: 1,
			Text: &TextData{
				Data: "note",
			},
		},
	}

	resp := &SyncRequest{
		Items: items,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result SyncRequest
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Len(t, result.Items, len(items))
}

func TestSyncResponse_MarshalUnmarshal(t *testing.T) {
	now := time.Now()
	items := []Item{
		{
			ID:        uuid.New().String(),
			Type:      DataTypeText,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	resp := &SyncResponse{
		Items:      items,
		Conflicts:  []Item{},
		LastSyncAt: now,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result SyncResponse
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Len(t, result.Items, len(items))
	assert.Empty(t, result.Conflicts)
	assert.Equal(t, now.Unix(), result.LastSyncAt.Unix())
}

func TestRegisterRequest(t *testing.T) {
	req := &RegisterRequest{
		Email:    "test@example.com",
		Password: "secure-password-123",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var result RegisterRequest
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, req.Email, result.Email)
	assert.Equal(t, req.Password, result.Password)
}

func TestRegisterResponse(t *testing.T) {
	resp := &RegisterResponse{
		UserID:    uuid.New().String(),
		Email:     "test@example.com",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result RegisterResponse
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, resp.UserID, result.UserID)
	assert.Equal(t, resp.Email, result.Email)
	assert.Equal(t, resp.CreatedAt, result.CreatedAt)
}

func TestLoginRequest(t *testing.T) {
	req := &LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var result LoginRequest
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, req.Email, result.Email)
	assert.Equal(t, req.Password, result.Password)
}

func TestLoginResponse(t *testing.T) {
	user := &User{
		ID:        uuid.New().String(),
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := &LoginResponse{
		Token: "jwt-token",
		User:  user,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result LoginResponse
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, resp.Token, result.Token)
	assert.Equal(t, resp.User.ID, result.User.ID)
	assert.Equal(t, resp.User.Email, result.User.Email)
}

func TestErrorResponse(t *testing.T) {
	resp := &ErrorResponse{
		Error:   "validation_error",
		Message: "Email is required",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var result ErrorResponse
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, resp.Error, result.Error)
	assert.Equal(t, resp.Message, result.Message)
}
