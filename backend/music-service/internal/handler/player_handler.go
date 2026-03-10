package handler

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// spotifyPlayerURL es la base de todos los endpoints de control de reproducción de Spotify.
const spotifyPlayerURL = "https://api.spotify.com/v1/me/player"

// PlayerHandler actúa como proxy entre el frontend y la Spotify Web API.
// No valida el token de Spotify — si es inválido, Spotify devuelve 401 y se propaga al cliente.
type PlayerHandler struct {
	httpClient *http.Client
}

func NewPlayerHandler() *PlayerHandler {
	return &PlayerHandler{httpClient: &http.Client{}}
}

// playRequest contiene el track URI que se quiere reproducir.
// No incluye device_id porque el Web Playback SDK ya registra el dispositivo activo.
type playRequest struct {
	URIs []string `json:"uris"`
}

// Play recibe el URI del track y lo reproduce en el dispositivo activo del usuario.
// El dispositivo activo es el registrado por el Web Playback SDK en el browser.
func (h *PlayerHandler) Play(c *gin.Context) {
	var req playRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid JSON payload", nil))
		return
	}

	body, _ := json.Marshal(req)
	h.proxyToSpotify(c, http.MethodPut, spotifyPlayerURL+"/play", body)
}

// Pause pausa la reproducción en el dispositivo activo del usuario.
func (h *PlayerHandler) Pause(c *gin.Context) {
	h.proxyToSpotify(c, http.MethodPut, spotifyPlayerURL+"/pause", nil)
}

// Next salta a la siguiente canción en la cola de reproducción.
func (h *PlayerHandler) Next(c *gin.Context) {
	h.proxyToSpotify(c, http.MethodPost, spotifyPlayerURL+"/next", nil)
}

// Previous vuelve a la canción anterior en la cola de reproducción.
func (h *PlayerHandler) Previous(c *gin.Context) {
	h.proxyToSpotify(c, http.MethodPost, spotifyPlayerURL+"/previous", nil)
}

// proxyToSpotify es el método central que reenvía cualquier comando a la Spotify Web API.
// Extrae el token de Spotify del header Authorization enviado por el frontend
// y lo adjunta al request saliente. La respuesta de Spotify se propaga directamente al cliente.
func (h *PlayerHandler) proxyToSpotify(c *gin.Context, method, url string, body []byte) {
	// El frontend envía: Authorization: Bearer <spotify_access_token>
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, errorResponse("missing Authorization header", nil))
		return
	}

	// Spotify requiere un body vacío (no nil) en requests sin payload (pause, next, previous)
	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	} else {
		reqBody = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), method, url, reqBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("failed to build spotify request", nil))
		return
	}

	// Reenviar el token de Spotify tal cual — no se modifica ni valida localmente
	req.Header.Set("Authorization", authHeader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, errorResponse("failed to reach Spotify", nil))
		return
	}
	defer resp.Body.Close()

	// Spotify devuelve 204 No Content en operaciones exitosas sin body (pause, next, previous)
	if resp.StatusCode == http.StatusNoContent {
		c.Status(http.StatusNoContent)
		return
	}

	// Para cualquier otro status (errores 4xx/5xx de Spotify), propagar la respuesta tal cual
	var spotifyResp any
	if err := json.NewDecoder(resp.Body).Decode(&spotifyResp); err != nil {
		c.Status(resp.StatusCode)
		return
	}
	c.JSON(resp.StatusCode, spotifyResp)
}
