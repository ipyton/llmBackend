package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/sashabaranov/go-openai"
	"llm_servuce/entities"
	"net/http"
	"time"
)

type Chat struct {
	session *gocql.Session
	client  *openai.Client
}

func (chat *Chat) getHistorySessions(uuid string) []entities.Session {
	scanner := chat.session.Query("select sessionId, lastModified, title from botchats.sessions where uuid = ?", uuid).WithContext(context.Background()).Iter().Scanner()
	result := make([]entities.Session, 10)
	for scanner.Next() {
		var (
			sessionId    string
			lastModified time.Time
			title        string
		)
		err := scanner.Scan(&sessionId, &lastModified, &title)
		if err != nil {
			return nil
		}
		result = append(result, entities.Session{SessionId: sessionId, LastModified: lastModified, Title: title})
	}
	return result
}

func (chat *Chat) getHistoryChats(uuid string, sessionId gocql.UUID) []openai.ChatCompletionMessage {
	scanner := chat.session.Query("select * from botchats.chats where uuid = ? and sessionid = ?", uuid, sessionId).WithContext(context.Background()).Iter().Scanner()
	messages := make([]openai.ChatCompletionMessage, 10)
	for scanner.Next() {
		var (
			sessionId string
			sendTime  time.Time
			role      string
			content   string
		)
		fmt.Print(scanner.Scan(&sessionId, &sendTime, &role, &content))
		messages = append(messages, openai.ChatCompletionMessage{Role: role, Content: content})
	}
	return messages
}

func (chat *Chat) addNewChat(userId string, sessionId gocql.UUID, content string) (time.Time, string, error) {
	now := time.Now()
	err := chat.session.Query("insert into botChats.chats (sendTime, role, content) values (?,?,?) where userId = ? and sessionId = ?", time.Now(), "user", content, userId, sessionId).Exec()
	chats := chat.getHistoryChats(userId, sessionId)
	request := openai.ChatCompletionRequest{Model: "", Messages: chats}
	response, err := chat.client.CreateChatCompletion(context.Background(), request)
	chats = append(chats, openai.ChatCompletionMessage{Role: "user", Content: content})
	choice := response.Choices[0]
	chat.addResponse(userId, sessionId, choice.Message.Content)
	return now, choice.Message.Content, err
}

func (chat *Chat) addResponse(userId string, sessionId gocql.UUID, content string) error {
	err := chat.session.Query("insert into botChats.chats (sendTime, role, content) values (?,?,?) where userId = ? and sessionId = ?", time.Now(), "system", content, userId, sessionId).Exec()
	return err
}

func (chat *Chat) addNewSession(userId string, content string) (gocql.UUID, time.Time, string, error) {
	randomSessionId := gocql.MustRandomUUID()
	now := time.Now()
	err := chat.session.Query("insert into botChats.sessions (uuid, sessionId, lastModified, title) values (?,?,?,?)", userId, randomSessionId, now, content).Exec()
	chat.addNewChat(userId, randomSessionId, content)
	requestParams := make([]openai.ChatCompletionMessage, 10)
	requestParams = append(requestParams, openai.ChatCompletionMessage{Role: "user", Content: content})

	request := openai.ChatCompletionRequest{Model: "", Messages: requestParams}
	response, err := chat.client.CreateChatCompletion(context.Background(), request)
	choice := response.Choices[0]
	println(choice.Message.Content)
	chat.addResponse(userId, randomSessionId, choice.Message.Content)
	return randomSessionId, now, choice.Message.Content, err
}

func newChat() *Chat {
	cluster := gocql.NewCluster("192.168.2.75:9042")
	session, _ := cluster.CreateSession()
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: "cassandra",
		Password: "cassandra",
	}
	config := openai.DefaultConfig("sk-no-key-required")
	config.BaseURL = "http://192.168.2.75:8080/v1"
	client := openai.NewClientWithConfig(config)
	return &Chat{session: session, client: client}
}

func main() {
	server := gin.Default()

	chat := newChat()

	server.POST("/get_sessions", func(c *gin.Context) {
		value := c.PostForm("uuid")
		result := chat.getHistorySessions(value)

		marshal, err := json.Marshal(result)
		if err != nil {
			c.String(http.StatusOK, string(marshal))
		}
	})

	server.POST("/get_chats", func(c *gin.Context) {
		uuid := c.PostForm("uuid")
		sessionId := c.PostForm("sessionid")

		bytes, err2 := gocql.UUIDFromBytes([]byte(sessionId))
		if err2 != nil {
			result := chat.getHistoryChats(uuid, bytes)
			print(uuid)
			println(sessionId)
			marshal, err := json.Marshal(result)
			if err != nil {
				c.String(http.StatusOK, string(marshal))
			}
		}
	})

	server.POST("/add_session", func(c *gin.Context) {
		uuid := c.PostForm("uuid")

		content := c.PostForm("content")
		sessionId, sendTime, content, err := chat.addNewSession(uuid, content)
		marshal, err := json.Marshal(entities.Response{SendTime: sendTime, SessionID: sessionId, CompleteTime: time.Now(), Data: content})
		print(err)
		if err != nil {

		}
		print(string(marshal))
		c.String(http.StatusOK, string(marshal))
	})

	server.POST("/continue_chat", func(c *gin.Context) {
		uuid := c.PostForm("uuid")
		sessionId := c.PostForm("sessionid")
		content := c.PostForm("content")
		bytes, err := gocql.UUIDFromBytes([]byte(sessionId))
		if err != nil {

		}
		sendTime, response, err1 := chat.addNewChat(uuid, bytes, content)
		marshal, err2 := json.Marshal(entities.Response{SendTime: sendTime, CompleteTime: time.Now(), Data: response})
		if err1 != nil && err2 != nil {
		}
		c.String(http.StatusOK, string(marshal))

	})

	server.Run("0.0.0.0:9090")
}
