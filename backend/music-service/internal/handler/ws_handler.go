package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// spotifyPlayerURL es la base de todos los endpoints de control de reproducción de Spotify.
const spotifyPlayerURL = "https://api.spotify.com/v1/me/player"

// upgrader convierte una conexión HTTP en WebSocket.
// CheckOrigin valida que el request venga del frontend autorizado.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "http://localhost:5173"
	},
}

// wsInMessage representa un comando enviado por el frontend al backend.
type wsInMessage struct {
	Action string `json:"action"` // play | pause | next | previous
	URI    string `json:"uri"`    // spotify:track:ID — solo requerido para "play"
}

// wsOutMessage representa la respuesta del backend al frontend.
type wsOutMessage struct {
	Event   string `json:"event"`             // ok | error
	Action  string `json:"action"`            // acción que originó el evento
	Message string `json:"message,omitempty"` // detalle del error si aplica
}

// WSHandler gestiona la conexión WebSocket de una sesión de entrenamiento.
// Recibe comandos del frontend, los ejecuta contra la Spotify Web API
// y devuelve el resultado por el mismo canal.
type WSHandler struct {
	httpClient *http.Client
}

func NewWSHandler() *WSHandler {
	return &WSHandler{httpClient: &http.Client{}}
}

// HandleSession upgradea la conexión HTTP a WebSocket y mantiene el canal abierto
// durante toda la sesión de entrenamiento.
//
// El frontend se conecta así:
//
//	ws://localhost:8081/api/v1/ws?token=<spotify_access_token>
//
// Una vez conectado, envía mensajes JSON con la forma:
//
//	{ "action": "play", "uri": "spotify:track:ID" }
//	{ "action": "pause" }
//	{ "action": "next" }
//	{ "action": "previous" }
//
// El token se pasa como query param porque los browsers no permiten
// enviar headers Authorization en la apertura de una conexión WebSocket.
func (h *WSHandler) HandleSession(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, errorResponse("missing token query param", nil))
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// gorilla escribe automáticamente el error HTTP si el upgrade falla
		return
	}
	defer conn.Close()

	authHeader := "Bearer " + token

	// Bucle de mensajes: se mantiene activo hasta que el cliente cierra la conexión
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			// Error de lectura o cierre normal del cliente (websocket.CloseMessage)
			break
		}

		var msg wsInMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			h.writeOut(conn, wsOutMessage{Event: "error", Message: "invalid message format"})
			continue
		}

		if err := h.dispatch(conn, msg, authHeader); err != nil {
			h.writeOut(conn, wsOutMessage{Event: "error", Action: msg.Action, Message: err.Error()})
		}
	}
}

// dispatch selecciona qué llamado a Spotify corresponde a cada acción recibida.
func (h *WSHandler) dispatch(conn *websocket.Conn, msg wsInMessage, authHeader string) error {
	switch msg.Action {
	case "play":
		if strings.TrimSpace(msg.URI) == "" {
			return fmt.Errorf("uri is required for play action")
		}
		body := map[string]any{"uris": []string{msg.URI}}
		return h.callSpotify(conn, msg.Action, http.MethodPut, spotifyPlayerURL+"/play", authHeader, body)
	case "pause":
		return h.callSpotify(conn, msg.Action, http.MethodPut, spotifyPlayerURL+"/pause", authHeader, nil)
	case "next":
		return h.callSpotify(conn, msg.Action, http.MethodPost, spotifyPlayerURL+"/next", authHeader, nil)
	case "previous":
		return h.callSpotify(conn, msg.Action, http.MethodPost, spotifyPlayerURL+"/previous", authHeader, nil)
	default:
		return fmt.Errorf("unknown action: %s", msg.Action)
	}
}

// callSpotify ejecuta la petición HTTP a la Spotify Web API y escribe el resultado
// de vuelta al cliente por el canal WebSocket.
func (h *WSHandler) callSpotify(conn *websocket.Conn, action, method, url, authHeader string, body map[string]any) error {
	var reqBody *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	} else {
		// Spotify requiere body vacío (no nil) en pause, next, previous
		reqBody = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to build spotify request")
	}

	req.Header.Set("Authorization", authHeader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach Spotify")
	}
	defer resp.Body.Close()

	// 204 No Content = operación exitosa sin body de respuesta
	if resp.StatusCode == http.StatusNoContent {
		h.writeOut(conn, wsOutMessage{Event: "ok", Action: action})
		return nil
	}

	// Cualquier otro status de Spotify se trata como error y se propaga al cliente
	var spotifyErr any
	json.NewDecoder(resp.Body).Decode(&spotifyErr)
	return fmt.Errorf("spotify returned %d", resp.StatusCode)
}

// writeOut serializa y envía un mensaje JSON por el canal WebSocket.
func (h *WSHandler) writeOut(conn *websocket.Conn, msg wsOutMessage) {
	b, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, b)
}
