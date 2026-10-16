package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	chromem "github.com/philippgille/chromem-go"
)

func createTestChromaDB(t *testing.T, path string, contents ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`CREATE TABLE embeddings (id INTEGER PRIMARY KEY, embedding_id TEXT)`,
		`CREATE TABLE embedding_metadata (id INTEGER, key TEXT, string_value TEXT)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for i, content := range contents {
		if _, err := db.Exec(`INSERT INTO embeddings (id, embedding_id) VALUES (?, ?)`, i+1, fmt.Sprintf("fragment-%d", i+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO embedding_metadata (id, key, string_value) VALUES (?, 'chroma:document', ?)`, i+1, content); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadChromaDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chroma.sqlite3")
	createTestChromaDB(t, path, "Una regla de Vampiro V20.", "   ")
	documents, err := readChromaDocuments(path)
	if err != nil || len(documents) != 1 || documents[0].id != "fragment-1" {
		t.Fatalf("documents = %#v, error = %v", documents, err)
	}
}

func TestSearchWhileIndexingReturnsServiceUnavailable(t *testing.T) {
	app := fiber.New()
	app.Post("/api/search", (&appState{}).search)
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(`{"question":"¿Cómo funciona esta regla?"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestEmbeddingEndpointRejectsHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<!doctype html><title>Endpoint equivocado</title>")
	}))
	defer server.Close()
	_, err := newEmbeddingFunc(server.URL+"/v1", "", "test-model", server.Client())(context.Background(), "prueba")
	if err == nil || !strings.Contains(err.Error(), `"text/html"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDegenerateFragmentsAreNotSearchable(t *testing.T) {
	for _, junk := range []string{"OZ", "132", "Melpómene", "VAMPIRO LA MASCARADA EDICIÓN 20? ANIVERSARIO\n159", "e.... e Alma Compartida"} {
		if isSearchableFragment(junk) {
			t.Errorf("accepted junk fragment %q", junk)
		}
	}
}

func TestAllRulesQuestionsUseModelAndKeepSourcesOutsideSystemPrompt(t *testing.T) {
	const manual = "Capítulo de prueba · Efecto de Prueba · pág. 10\nEl efecto usa una reserva variable y no requiere gasto. No transfieras a este efecto la tirada del ejemplo que aparece en otro apartado."
	questions := []string{
		"¿Qué tirada hago para este efecto?",
		"¿Qué puedo usar con dos puntos de esta Disciplina?",
		"¿Qué ramas de esta tradición existen?",
		"¿Cómo funciona el combate?",
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/embeddings" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"embedding":[1,0]}]}`)
			return
		}
		var request chatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		calls++
		if len(request.Messages) != 2 || request.Messages[0].Content != rulesSystemPrompt {
			t.Errorf("system prompt differs across questions: %#v", request.Messages)
		}
		if !strings.Contains(request.Messages[1].Content, manual) || !strings.Contains(request.Messages[1].Content, "[1]") {
			t.Error("missing original numbered manual source")
		}
		if !request.Stream || request.ChatTemplateKwargs["enable_thinking"] != false {
			t.Error("expected direct streamed response")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Respuesta del modelo [1].\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "chroma.sqlite3")
	createTestChromaDB(t, path, manual)
	state, err := newAppState(config{
		chromaPath: path, chromemPath: filepath.Join(t.TempDir(), "chromem"),
		llmURL: server.URL + "/v1", embedURL: server.URL + "/v1",
		llmModel: "test", embedModel: "test", minSimilarity: 0.22,
	})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/api/search", state.search)
	for _, question := range questions {
		body, _ := json.Marshal(searchRequest{Question: question})
		request := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(string(body)))
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if err != nil {
			t.Fatal(err)
		}
		payload, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || !strings.Contains(string(payload), "Respuesta del modelo") || !strings.Contains(string(payload), `"type":"done"`) {
			t.Fatalf("question %q: response = %s, error = %v", question, payload, err)
		}
	}
	if calls != len(questions) {
		t.Fatalf("got %d model calls, want %d", calls, len(questions))
	}
}

func TestRetrievalPreservesFormulasAndAdjacentExceptions(t *testing.T) {
	documents := []chromaDocument{
		{id: "power", content: "Reglas · Poder de prueba · pág. 1\nEste Poder requiere una tirada. La dificultad es la Fuerza de Voluntad temporal del objetivo."},
		{id: "exception", content: "Reglas · Poder de prueba · pág. 2\nSi el objetivo acepta el contacto no hay tirada. Esta excepción tiene prioridad sobre la regla general."},
		{id: "unrelated", content: "Otro capítulo · Ejemplo · pág. 3\nPara este ejemplo distinto, el personaje tira Destreza + Atletismo a dificultad 6."},
	}
	positions := map[string]int{"power": 0, "exception": 1, "unrelated": 2}
	context, sources := retrieveManualContext("¿Cómo funciona el Poder de prueba?", []chromem.Result{{ID: "power", Similarity: 0.8}}, documents, positions, 0.22)
	if !strings.Contains(context, documents[0].content) || !strings.Contains(context, documents[1].content) || strings.Contains(context, documents[2].content) {
		t.Fatalf("lost formula/exception or crossed chapter: %s", context)
	}
	if len(sources) != 2 {
		t.Fatalf("got %d sources", len(sources))
	}
}

func TestProvidedManualRetrievalCoversPlayerQuestions(t *testing.T) {
	documents, err := readChromaDocuments("./v20_db/chroma.sqlite3")
	if err != nil {
		t.Fatal(err)
	}
	positions := make(map[string]int)
	for i, document := range documents {
		positions[document.id] = i
	}
	for _, test := range []struct {
		question string
		want     []string
	}{
		{"que sendas o tipos de ramas de la taumaturgia existe?", []string{"Conjuración, Senda de la", "Control del Clima", "Corrupción, Senda de la", "Dominio Elemental", "Encanto de las Llamas", "Manos de Destrucción", "Marte, Senda de", "Movimiento Mental", "Poder de Neptuno", "Sangre, Senda de la", "Tecnomancia, Senda de la", "Venganza del Padre, Senda de la", "Verde, Senda"}},
		{"¿Qué sendas de Necromancia existen?", []string{"Sendas:", "Sepulcro", "Cenizas", "Hueso"}},
		{"¿Qué puedo usar con 2 puntos de Auspex y qué hace cada poder?", []string{"Sentidos Agudizados", "Percepción del Aura", "dificultad 8", "no se necesita tirada"}},
		{"¿Cómo funciona Telepatía y cómo resiste el objetivo?", []string{"Inteligencia + Subterfugio", "Fuerza de Voluntad temporal del sujeto", "disfrazar"}},
		{"¿Cómo funciona la absorción de daño?", []string{"absorción", "Resistencia", "Fortaleza"}},
		{"¿Cómo se resuelve una tirada enfrentada?", []string{"enfrentada", "fuerza de voluntad", "éxitos"}},
		{"¿Qué coste de experiencia tiene aprender una Disciplina?", []string{"Experiencia", "Disciplina"}},
	} {
		t.Run(test.question, func(t *testing.T) {
			// Sin vectores: también se comprueba que los índices y títulos
			// explícitos no dependan de un acierto del servicio de embeddings.
			context, sources := retrieveManualContext(test.question, nil, documents, positions, 0.22)
			for _, want := range test.want {
				if !strings.Contains(normalizeWords(context), normalizeWords(want)) {
					t.Errorf("missing %q in %d sources (%d bytes)", want, len(sources), len(context))
				}
			}
			if len(context) > maxManualContext {
				t.Fatalf("context exceeds budget: %d", len(context))
			}
		})
	}
}

func TestSectionPowerIndexUsesOnlyDeclaredLevels(t *testing.T) {
	documents := []chromaDocument{
		{id: "a", content: "Reglas · Disciplina de prueba · pág. 1\nMedio mundo de la disciplina."},
		{id: "b", content: "Reglas · Disciplina de prueba · pág. 1\nDisciplina de prueba 1 o más\ne Poder Alfa"},
		{id: "c", content: "Reglas · Disciplina de prueba · pág. 2\ne Poder Beta"},
		{id: "d", content: "Reglas · Disciplina de prueba · pág. 2\nDisciplina de prueba 5 o más\ne Poder Gamma"},
		{id: "e", content: "Reglas · Otra sección · pág. 3\nOtra sección 1 o más\ne Poder Delta"},
	}
	index := sectionPowerIndex("¿Qué Poderes tiene la Disciplina de prueba?", documents)
	for _, want := range []string{"Nivel 1: Poder Alfa", "Nivel no indicado en el pasaje: Poder Beta", "Nivel 5: Poder Gamma"} {
		if !strings.Contains(index, want) {
			t.Errorf("missing %q in:\n%s", want, index)
		}
	}
	if strings.Contains(index, "Poder Delta") {
		t.Errorf("crossed into another section:\n%s", index)
	}
	if strings.Contains(index, "Nivel 2:") {
		t.Errorf("invented a level by counting:\n%s", index)
	}
}

func TestStreamDoesNotReportTruncatedAnswersAsComplete(t *testing.T) {
	for _, payload := range []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Respuesta parcial\"},\"finish_reason\":\"length\"}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"Respuesta parcial\"}}]}\n\n",
	} {
		response := &http.Response{Body: io.NopCloser(strings.NewReader(payload))}
		if err := pumpComplete(response, func(string) error { return nil }); err == nil {
			t.Fatal("truncated answer was accepted as complete")
		}
	}
}
