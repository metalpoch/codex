package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	chromem "github.com/philippgille/chromem-go"
	_ "modernc.org/sqlite"
)

const (
	defaultModel       = "unsloth/Qwen3.5-4B-GGUF:Q4_K_M"
	defaultEmbedder    = "Qwen/Qwen3-Embedding-0.6B-GGUF:F16"
	collectionName     = "vampiro-v20"
	maxQuestionChars   = 700
	minSearchableRunes = 60
)

//go:embed web/index.html web/app.css web/app.js web/favicon.svg web/temps.min.js
var web embed.FS

var wordPattern = regexp.MustCompile(`[^a-z0-9]+`)

type config struct {
	chromaPath    string
	chromemPath   string
	llmURL        string
	llmModel      string
	embedURL      string
	embedModel    string
	apiKey        string
	embedAPIKey   string
	port          string
	minSimilarity float32
}

type appState struct {
	config            config
	collection        *chromem.Collection
	client            *http.Client
	sourceDocuments   []chromaDocument
	documentPositions map[string]int
	ready             atomic.Bool
}

type searchRequest struct {
	Question string `json:"question"`
}

type source struct {
	Excerpt    string  `json:"excerpt"`
	Similarity float32 `json:"similarity"`
}

type searchResponse struct {
	Answer  string   `json:"answer"`
	Sources []source `json:"sources"`
}

type chromaDocument struct {
	id      string
	content string
}

type chatRequest struct {
	Model              string         `json:"model"`
	Messages           []chatMessage  `json:"messages"`
	Temperature        float64        `json:"temperature"`
	MaxTokens          int            `json:"max_tokens"`
	Stream             bool           `json:"stream,omitempty"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

var noThinking = map[string]any{"enable_thinking": false}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--embeddings" {
			log.Fatal("--embeddings es una opción del servidor llama.cpp, no de Codex. Añádela al proceso que sirve LLM_BASE_URL y arranca Codex con `go run .`.")
		}
	}
	cfg := loadConfig()
	state, err := openAppState(cfg)
	if err != nil {
		log.Fatalf("No se pudo preparar Codex: %v", err)
	}
	app := fiber.New(fiber.Config{AppName: "Codex · V20"})
	for _, asset := range []struct{ route, path, kind string }{
		{"/", "web/index.html", "html"},
		{"/favicon.svg", "web/favicon.svg", "svg"},
		{"/app.css", "web/app.css", "css"},
		{"/app.js", "web/app.js", "js"},
		{"/temps.min.js", "web/temps.min.js", "js"},
	} {
		content, err := web.ReadFile(asset.path)
		if err != nil {
			log.Fatal(err)
		}
		app.Get(asset.route, func(c fiber.Ctx) error {
			c.Type(asset.kind, "utf-8")
			return c.Send(content)
		})
	}
	app.Get("/api/health", func(c fiber.Ctx) error {
		if !state.ready.Load() {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "indexing", "documents": 0})
		}
		return c.JSON(fiber.Map{"status": "ok", "documents": state.collection.Count()})
	})
	app.Post("/api/search", state.search)
	go func() {
		if err := state.importIndex(); err != nil {
			log.Fatalf("No se pudo indexar el archivo: %v", err)
		}
		log.Printf("Índice listo: %d fragmentos", state.collection.Count())
	}()
	log.Printf("Codex escuchando en http://localhost:%s · el índice se prepara en segundo plano", cfg.port)
	log.Fatal(app.Listen(":" + cfg.port))
}

func loadConfig() config {
	return config{
		chromaPath:    env("CHROMA_DB_PATH", "./v20_db/chroma.sqlite3"),
		chromemPath:   env("CHROMEM_DB_PATH", "./codex_data/chromem"),
		llmURL:        trimURL(env("LLM_BASE_URL", "http://192.168.100.100:8080/v1")),
		llmModel:      env("LLM_MODEL", defaultModel),
		embedURL:      trimURL(env("EMBEDDING_BASE_URL", "http://192.168.100.100:8080/v1")),
		embedModel:    env("EMBEDDING_MODEL", defaultEmbedder),
		apiKey:        env("LLM_API_KEY", "no-key-required"),
		embedAPIKey:   env("EMBEDDING_API_KEY", env("LLM_API_KEY", "no-key-required")),
		port:          env("PORT", "3000"),
		minSimilarity: 0.22,
	}
}

func newAppState(cfg config) (*appState, error) {
	state, err := openAppState(cfg)
	if err != nil {
		return nil, err
	}
	if err := state.importIndex(); err != nil {
		return nil, err
	}
	return state, nil
}

func openAppState(cfg config) (*appState, error) {
	if _, err := os.Stat(cfg.chromaPath); err != nil {
		return nil, fmt.Errorf("no se encuentra la base ChromaDB en %q: %w", cfg.chromaPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.chromemPath), 0o755); err != nil {
		return nil, fmt.Errorf("no se pudo crear el directorio de chromem-go: %w", err)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	embedding := newEmbeddingFunc(cfg.embedURL, cfg.embedAPIKey, cfg.embedModel, client)
	db, err := chromem.NewPersistentDB(cfg.chromemPath, true)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir chromem-go: %w", err)
	}
	collection, err := db.GetOrCreateCollection(collectionName, map[string]string{"source": "ChromaDB Vampiro V20"}, embedding)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la colección V20: %w", err)
	}
	sourceDocuments, err := readChromaDocuments(cfg.chromaPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los textos de ChromaDB: %w", err)
	}
	documentPositions := make(map[string]int, len(sourceDocuments))
	for position, document := range sourceDocuments {
		documentPositions[document.id] = position
	}
	return &appState{
		config: cfg, collection: collection, client: client,
		sourceDocuments: sourceDocuments, documentPositions: documentPositions,
	}, nil
}

func (s *appState) importIndex() error {
	if err := importChromaDocuments(s.sourceDocuments, s.collection, s.config.embedModel); err != nil {
		return err
	}
	s.ready.Store(true)
	return nil
}

func newEmbeddingFunc(baseURL, apiKey, model string, client *http.Client) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		body, err := json.Marshal(map[string]string{"input": text, "model": model})
		if err != nil {
			return nil, fmt.Errorf("no se pudo preparar la petición de embeddings: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("no se pudo crear la petición a %s/embeddings: %w", baseURL, err)
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("no se pudo conectar con %s/embeddings: %w", baseURL, err)
		}
		defer response.Body.Close()
		responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		if err != nil {
			return nil, fmt.Errorf("no se pudo leer la respuesta de embeddings: %w", err)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("%s/embeddings respondió %s: %s", baseURL, response.Status, responsePreview(responseBody))
		}
		contentType := response.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(contentType), "json") {
			return nil, fmt.Errorf("%s/embeddings devolvió %q en lugar de JSON: %s", baseURL, contentType, responsePreview(responseBody))
		}
		var result embeddingResponse
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return nil, fmt.Errorf("respuesta JSON inválida de %s/embeddings: %w; cuerpo: %s", baseURL, err, responsePreview(responseBody))
		}
		if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
			return nil, fmt.Errorf("%s/embeddings no devolvió ningún vector", baseURL)
		}
		return result.Data[0].Embedding, nil
	}
}

func responsePreview(body []byte) string {
	preview := strings.TrimSpace(string(body))
	if len(preview) > 300 {
		preview = preview[:300] + "…"
	}
	return preview
}

func importChromaDocuments(documents []chromaDocument, collection *chromem.Collection, embeddingModel string) error {
	if len(documents) == 0 {
		return errors.New("la colección de ChromaDB no contiene fragmentos de texto")
	}
	missing := make([]chromem.Document, 0)
	degenerate := make([]string, 0)
	for _, document := range documents {
		if !isSearchableFragment(document.content) {
			if _, err := collection.GetByID(context.Background(), document.id); err == nil {
				degenerate = append(degenerate, document.id)
			}
			continue
		}
		existing, err := collection.GetByID(context.Background(), document.id)
		if err == nil {
			if storedModel := existing.Metadata["embedding_model"]; storedModel != "" && storedModel != embeddingModel {
				return fmt.Errorf("la colección persistida usa el modelo de embeddings %q, pero la configuración indica %q; usa el mismo modelo o cambia CHROMEM_DB_PATH", storedModel, embeddingModel)
			}
			continue
		}
		missing = append(missing, chromem.Document{
			ID: document.id, Content: document.content,
			Metadata: map[string]string{"source": "Vampiro: La Mascarada V20", "embedding_model": embeddingModel},
		})
	}
	if len(degenerate) > 0 {
		if err := collection.Delete(context.Background(), nil, nil, degenerate...); err != nil {
			return fmt.Errorf("no se pudieron retirar los fragmentos inservibles de chromem-go: %w", err)
		}
		log.Printf("Retirados %d fragmentos sin contenido útil de la colección", len(degenerate))
	}
	if len(missing) == 0 {
		log.Printf("Colección chromem-go lista: %d fragmentos", collection.Count())
		return nil
	}
	log.Printf("Importando %d fragmentos de ChromaDB a chromem-go. La primera carga calcula embeddings y puede tardar varios minutos...", len(missing))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	if err := collection.AddDocuments(ctx, missing, 6); err != nil {
		return fmt.Errorf("falló la importación a chromem-go (comprueba el endpoint %s/embeddings y el modelo %q; su preset debe incluir embedding=true y pooling=last): %w", env("EMBEDDING_BASE_URL", "http://192.168.100.100:8080/v1"), env("EMBEDDING_MODEL", defaultEmbedder), err)
	}
	log.Printf("Importación completada: %d fragmentos", collection.Count())
	return nil
}

func readChromaDocuments(path string) ([]chromaDocument, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(absolutePath)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	rows, err := db.Query(`
		SELECT e.embedding_id, m.string_value
		FROM embeddings AS e
		JOIN embedding_metadata AS m ON m.id = e.id
		WHERE m.key = 'chroma:document' AND m.string_value IS NOT NULL
		ORDER BY e.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var documents []chromaDocument
	for rows.Next() {
		var document chromaDocument
		if err := rows.Scan(&document.id, &document.content); err != nil {
			return nil, err
		}
		if strings.TrimSpace(document.content) != "" {
			documents = append(documents, document)
		}
	}
	return documents, rows.Err()
}

func (s *appState) search(c fiber.Ctx) error {
	if !s.ready.Load() {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "El archivo aún se está indexando. Prueba de nuevo en unos minutos."})
	}
	var request searchRequest
	if err := json.Unmarshal(c.Body(), &request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Envía una pregunta válida."})
	}
	question := strings.TrimSpace(request.Question)
	if question == "" || len([]rune(question)) > maxQuestionChars {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "La pregunta debe tener entre 1 y 700 caracteres."})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	k := 12
	if asksForBroadCoverage(question) {
		k = 24
	}
	results, err := s.collection.Query(ctx, question, min(k, s.collection.Count()), nil, nil)
	if err != nil {
		log.Printf("Error de búsqueda vectorial: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "No pude consultar el archivo. Comprueba el servicio de embeddings."})
	}
	manualContext, sources := retrieveManualContext(question, results, s.sourceDocuments, s.documentPositions, s.config.minSimilarity)
	if len(sources) == 0 {
		return c.JSON(searchResponse{Answer: "No encuentro en los fragmentos del manual información suficiente para responder a eso. Prueba con una pregunta más concreta sobre V20."})
	}
	llmCtx, llmCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	upstream, err := s.startComplete(llmCtx, question, manualContext)
	if err != nil {
		llmCancel()
		log.Printf("Error del modelo local: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "El erudito no está disponible ahora. Comprueba el modelo local."})
	}
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer llmCancel()
		writeEvent := func(value any) bool {
			encoded, err := json.Marshal(value)
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
				return false
			}
			return w.Flush() == nil
		}
		writeEvent(fiber.Map{"type": "sources", "sources": sources})
		sent := false
		err := pumpComplete(upstream, func(text string) error {
			if !writeEvent(fiber.Map{"type": "delta", "text": text}) {
				return errors.New("no se pudo escribir en la respuesta")
			}
			sent = true
			return nil
		})
		switch {
		case err != nil && !sent:
			writeEvent(fiber.Map{"type": "error", "message": "El erudito no está disponible ahora. Comprueba el modelo local."})
		case err != nil:
			writeEvent(fiber.Map{"type": "error", "message": "La respuesta del erudito se interrumpió a mitad de camino."})
		default:
			writeEvent(fiber.Map{"type": "done"})
		}
	})
}

func isSearchableFragment(content string) bool {
	text := strings.TrimSpace(content)
	return len([]rune(text)) >= minSearchableRunes && len(strings.Fields(text)) >= 8
}

func normalizeWords(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(value)
	return strings.TrimSpace(wordPattern.ReplaceAllString(value, " "))
}

func (s *appState) startComplete(ctx context.Context, question, manualContext string) (*http.Response, error) {
	maxTokens := 1200
	if asksForBroadCoverage(question) {
		maxTokens = 3200
	}
	body, err := json.Marshal(chatRequest{
		Model: s.config.llmModel,
		Messages: []chatMessage{
			{Role: "system", Content: rulesSystemPrompt},
			{Role: "user", Content: "Pasajes del manual (fuentes de consulta):\n" + manualContext + "\nPregunta del jugador:\n" + question},
		},
		Temperature: 0.05, MaxTokens: maxTokens, Stream: true, ChatTemplateKwargs: noThinking,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.llmURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.config.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.apiKey)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("endpoint del modelo respondió %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	return response, nil
}

func pumpComplete(response *http.Response, emit func(string) error) error {
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	finished := false
	for {
		line, err := reader.ReadString('\n')
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				return nil
			}
			var chunk chatStreamResponse
			if json.Unmarshal([]byte(payload), &chunk) == nil && len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				if choice.Delta.Content != "" {
					if emitErr := emit(choice.Delta.Content); emitErr != nil {
						return emitErr
					}
				}
				if choice.FinishReason == "length" {
					return errors.New("la respuesta alcanzó el límite de tokens")
				}
				finished = finished || choice.FinishReason == "stop"
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) && finished {
				return nil
			}
			return err
		}
	}
}

func trimURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
