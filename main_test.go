package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	chromem "github.com/philippgille/chromem-go"
)

func TestReadChromaDocuments(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chroma.sqlite3")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, statement := range []string{
		`CREATE TABLE embeddings (id INTEGER PRIMARY KEY, embedding_id TEXT)`,
		`CREATE TABLE embedding_metadata (id INTEGER, key TEXT, string_value TEXT)`,
		`INSERT INTO embeddings (id, embedding_id) VALUES (1, 'fragment-1')`,
		`INSERT INTO embedding_metadata (id, key, string_value) VALUES (1, 'chroma:document', 'Una regla de Vampiro V20.')`,
		`INSERT INTO embeddings (id, embedding_id) VALUES (2, 'fragment-empty')`,
		`INSERT INTO embedding_metadata (id, key, string_value) VALUES (2, 'chroma:document', '   ')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	documents, err := readChromaDocuments(dbPath)
	if err != nil {
		t.Fatalf("readChromaDocuments() error = %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("got %d documents, want 1", len(documents))
	}
	if documents[0].id != "fragment-1" || documents[0].content != "Una regla de Vampiro V20." {
		t.Fatalf("unexpected document: %#v", documents[0])
	}
}

func TestListQuestionLoadsAdjacentManualEntries(t *testing.T) {
	if !asksForList("¿Qué puedo lanzar con dos puntos de Taumaturgia?") {
		t.Fatal("expected a what-can-I-use question to trigger list retrieval")
	}
	if asksForList("¿Qué tirada requiere este Poder?") {
		t.Fatal("a single-mechanic question should not trigger list retrieval")
	}

	documents := []chromaDocument{
		{id: "path", content: "Senda de la Sangre"},
		{id: "power-1", content: "Sabor de la Sangre"},
		{id: "power-2", content: "Furia de Sangre"},
		{id: "power-3", content: "Sangre de Potencia"},
	}
	positions := map[string]int{"path": 0, "power-1": 1, "power-2": 2, "power-3": 3}
	query := []chromem.Result{{ID: "path", Content: "Senda de la Sangre", Similarity: 0.8}}
	context, sources := buildSearchContext(query, true, documents, positions, 0.22)
	for _, power := range []string{"Sabor de la Sangre", "Furia de Sangre", "Sangre de Potencia"} {
		if !strings.Contains(context, power) {
			t.Errorf("expanded manual context missing adjacent entry %q", power)
		}
	}
	if len(sources) != len(documents) {
		t.Fatalf("got %d source fragments, want %d", len(sources), len(documents))
	}
}

func TestThaumaturgyCatalogQuestionsFindTheManualIndex(t *testing.T) {
	question := "¿Qué sendas taumatúrgicas tienen los Tremere?"
	if !asksForList(question) || !asksForThaumaturgyCatalog(question) {
		t.Fatal("expected a Tremere Thaumaturgy catalog question to be recognized")
	}
	documents := []chromaDocument{
		{id: "intro", content: "La Taumaturgia se divide en Sendas y rituales."},
		{id: "index", content: "Taumaturgia, 82, 212-240 Sendas: Poder de Neptuno, Verde, Senda, rituales, 229-240."},
	}
	catalog, ok := thaumaturgyCatalogSource(documents)
	if !ok || !strings.Contains(catalog.Excerpt, "Poder de Neptuno") || !strings.Contains(catalog.Excerpt, "Verde, Senda") {
		t.Fatalf("catalog retrieval did not select the index passage: %#v", catalog)
	}
}

func TestDiceQuestionsRetrieveMoreRuleContext(t *testing.T) {
	question := "¿Qué tirada debo hacer para recordar algo que mi personaje sabe?"
	if !asksForDicePool(question) {
		t.Fatal("expected a dice-pool question to trigger additional retrieval")
	}
	if got := searchResultCount(question, 20); got != 12 {
		t.Fatalf("searchResultCount() = %d, want 12", got)
	}
	if got := searchResultCount("¿Qué puedo lanzar con 2 puntos de Auspex?", 20); got != 12 {
		t.Fatalf("list question should retrieve adjacent entries; got %d, want 12", got)
	}
}

func TestListSearchSplitsIndependentTopics(t *testing.T) {
	queries := alternativeSearchQueries("¿Existe una senda del agua y otra de las plantas para los Tremere?")
	if len(queries) != 2 || !strings.Contains(normalizeWords(strings.Join(queries, " ")), "agua") || !strings.Contains(normalizeWords(strings.Join(queries, " ")), "plantas") {
		t.Fatalf("expected independent water and plant retrieval queries, got %#v", queries)
	}
}

func TestMergeSearchResultsDeduplicatesByBestSimilarity(t *testing.T) {
	merged := mergeQueryResults(
		[]chromem.Result{{ID: "one", Similarity: 0.3}, {ID: "two", Similarity: 0.5}},
		[]chromem.Result{{ID: "one", Similarity: 0.8}, {ID: "three", Similarity: 0.4}},
		3,
	)
	if len(merged) != 3 || merged[0].ID != "one" || merged[0].Similarity != 0.8 {
		t.Fatalf("unexpected merged results: %#v", merged)
	}
}

func TestLexicalSupplementFindsRelatedAttributeDefinition(t *testing.T) {
	documents := []chromaDocument{
		{id: "merit", content: "Memoria Eidética es un Mérito; bajo estrés se tira Percepción + Alerta."},
		{id: "intelligence", content: "Atributos Mentales: la memoria y la inteligencia. Inteligencia mide hechos, conocimientos y razonamiento."},
	}
	query := "Atributos Mentales Inteligencia memoria hechos conocimientos"
	sources := lexicalManualSources(query, documents, nil, 3)
	if len(sources) == 0 || !strings.Contains(sources[0].Excerpt, "Inteligencia mide hechos") {
		t.Fatalf("lexical retrieval did not prioritize the matching Attribute definition: %#v", sources)
	}
}

func TestOrderedPathContextBuildsLevelListFromManualOrder(t *testing.T) {
	documents := []chromaDocument{
		{id: "path", content: "Senda de la Sangre\nCasi todos los Tremere estudian esta Senda."},
		{id: "one", content: "e Sabor de la Sangre\nEste Poder revela información sobre la Vitae."},
		{id: "two", content: "e Furia de Sangre\nEste Poder obliga a gastar Sangre."},
		{id: "three", content: "e Sangre de Potencia\nEste Poder cambia la Generación."},
		{id: "next-path", content: "Senda de la Conjuración\ne Invocar la Forma Sencilla"},
	}
	context := orderedSectionContext("¿Qué puedo lanzar con 2 puntos en la Senda de la Sangre?", documents)
	for _, expected := range []string{
		"Nivel 1: Sabor de la Sangre",
		"Nivel 2: Furia de Sangre",
		"Nivel 3: Sangre de Potencia",
	} {
		if !strings.Contains(context, expected) {
			t.Errorf("ordered path context missing %q: %s", expected, context)
		}
	}
	if strings.Contains(context, "Invocar la Forma Sencilla") {
		t.Fatal("ordered path context crossed into the following Senda")
	}
	options := context[strings.Index(context, "OPCIONES DISPONIBLES"):]
	if !strings.Contains(options, "Nivel 1: Sabor de la Sangre") || !strings.Contains(options, "Nivel 2: Furia de Sangre") {
		t.Fatalf("level-two options should include the first two powers: %s", options)
	}
	if strings.Contains(options, "Sangre de Potencia") {
		t.Fatalf("level-three power should not be in the two-dot options: %s", options)
	}
	answer, ok := formatAvailablePathAnswer("¿Qué puedo lanzar con 2 puntos en la Senda de la Sangre?", context)
	if !ok || !strings.Contains(answer, "Sabor de la Sangre") || !strings.Contains(answer, "Furia de Sangre") || strings.Contains(answer, "Sangre de Potencia") {
		t.Fatalf("unexpected derived option list: %s", answer)
	}
}

func TestOrderedDisciplineContextIsNotLimitedToSendas(t *testing.T) {
	documents := []chromaDocument{
		{id: "auspex", content: "Auspex\nDescripción de la Disciplina."},
		{id: "heightened", content: "e Sentidos Agudizados\nMejora los sentidos."},
		{id: "aura", content: "e Percepción del Aura\nPercibe el aura."},
		{id: "elder", content: "Auspex 6 o más\ne Reflejo Especular"},
		{id: "celerity", content: "Celeridad\nDescripción de Celeridad."},
	}
	context := orderedSectionContext("¿Qué Poderes de Auspex puedo usar con 2 puntos?", documents)
	answer, ok := formatAvailablePathAnswer("¿Qué Poderes de Auspex puedo usar con 2 puntos?", context)
	if !ok || !strings.Contains(answer, "Sentidos Agudizados (nivel 1)") || !strings.Contains(answer, "Percepción del Aura (nivel 2)") || strings.Contains(answer, "Reflejo Especular") {
		t.Fatalf("unexpected Auspex list: %s", answer)
	}
}

func TestProvidedBloodPathSequence(t *testing.T) {
	documents, err := readChromaDocuments("./v20_db/chroma.sqlite3")
	if err != nil {
		t.Skipf("manual database unavailable: %v", err)
	}
	catalogQuestion := "¿Qué sendas taumatúrgicas pueden seguir los Tremere y qué hace cada una?"
	if !asksForThaumaturgyCatalog(catalogQuestion) {
		t.Fatal("expected the Tremere Thaumaturgy catalog intent to be recognized")
	}
	catalog, ok := thaumaturgyCatalogSource(documents)
	if !ok || !strings.Contains(catalog.Excerpt, "Poder de Neptuno") || !strings.Contains(catalog.Excerpt, "Verde, Senda") {
		t.Fatalf("manual index missing water/plant path entries: %#v", catalog)
	}
	titles := thaumaturgyPathTitles(catalog.Excerpt)
	joinedTitles := strings.Join(titles, " | ")
	if !strings.Contains(joinedTitles, "Poder de Neptuno") || !strings.Contains(joinedTitles, "Senda Verde") {
		t.Fatalf("failed to normalize water/plant headings from index: %#v", titles)
	}
	context := orderedSectionContext("¿Qué puedo lanzar con 2 puntos de Taumaturgia en la Senda de la Sangre?", documents)
	for _, expected := range []string{
		"Nivel 1: Sabor de la Sangre",
		"Nivel 2: Furia de Sangre",
		"Nivel 3: Sangre de Potencia",
		"Nivel 4: Robo de Vitae",
		"Nivel 5: Caldero de Sangre",
	} {
		if !strings.Contains(context, expected) {
			t.Errorf("manual sequence missing %q", expected)
		}
	}
	options := context[strings.Index(context, "OPCIONES DISPONIBLES"):]
	if strings.Contains(options, "Sangre de Potencia") || strings.Contains(options, "Caldero de Sangre") {
		t.Fatalf("two-dot options included a higher-level power: %s", options)
	}
	answer, ok := formatAvailablePathAnswer("¿Qué puedo lanzar con 2 puntos de Taumaturgia en la Senda de la Sangre?", context)
	if !ok || !strings.Contains(answer, "Sabor de la Sangre (nivel 1)") || !strings.Contains(answer, "Furia de Sangre (nivel 2)") || strings.Contains(answer, "Sangre de Potencia") {
		t.Fatalf("incorrect list for the supplied V20 manual: %s", answer)
	}
}

func TestRequestedRating(t *testing.T) {
	for question, want := range map[string]int{
		"¿Qué puedo hacer con 2 puntos de Taumaturgia?":   2,
		"Lista los poderes disponibles hasta dos niveles": 2,
		"¿Qué tirada hago?":                               0,
	} {
		got, ok := requestedRating(question)
		if want == 0 {
			if ok {
				t.Errorf("requestedRating(%q) = %d, want no rating", question, got)
			}
		} else if !ok || got != want {
			t.Errorf("requestedRating(%q) = %d, %v; want %d", question, got, ok, want)
		}
	}
}

func TestSearchUsesRetrievedManualContext(t *testing.T) {
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/embeddings":
			_, _ = io.WriteString(w, `{"data":[{"embedding":[1,0]}]}`)
		case "/v1/chat/completions":
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Una respuesta fundada en el manual."}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer modelServer.Close()

	dbPath := filepath.Join(t.TempDir(), "chroma.sqlite3")
	createTestChromaDB(t, dbPath, "En V20, Percepción del Aura es un Poder de Auspex 2. El jugador tira Percepción + Empatía, dificultad 8.")
	cfg := config{
		chromaPath:    dbPath,
		chromemPath:   filepath.Join(t.TempDir(), "chromem"),
		llmURL:        modelServer.URL + "/v1",
		llmModel:      "test-model",
		embedURL:      modelServer.URL + "/v1",
		embedModel:    "test-embedding-model",
		apiKey:        "test-key",
		embedAPIKey:   "test-key",
		minSimilarity: 0.22,
	}
	state, err := newAppState(cfg)
	if err != nil {
		t.Fatalf("newAppState() error = %v", err)
	}
	app := fiber.New()
	app.Post("/api/search", state.search)

	request := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(`{"question":"¿Qué lanzamiento se necesita para ver las auras?"}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var result searchResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Answer, "Percepción + Empatía") || !strings.Contains(result.Answer, "8") || len(result.Sources) != 1 {
		t.Fatalf("unexpected search response: %#v", result)
	}
}

func TestSearchWhileIndexingReturnsServiceUnavailable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "chroma.sqlite3")
	createTestChromaDB(t, dbPath, "En V20, Percepción del Aura es un Poder de Auspex 2.")
	state, err := openAppState(config{
		chromaPath:    dbPath,
		chromemPath:   filepath.Join(t.TempDir(), "chromem"),
		embedURL:      "http://127.0.0.1:1/v1",
		embedModel:    "test-embedding-model",
		minSimilarity: 0.22,
	})
	if err != nil {
		t.Fatalf("openAppState() error = %v", err)
	}
	app := fiber.New()
	app.Get("/api/health", func(c fiber.Ctx) error {
		if !state.ready.Load() {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "indexing", "documents": 0})
		}
		return c.JSON(fiber.Map{"status": "ok", "documents": state.collection.Count()})
	})
	app.Post("/api/search", state.search)

	search := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(`{"question":"¿Qué es Percepción del Aura?"}`))
	search.Header.Set("Content-Type", "application/json")
	response, err := app.Test(search)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("search status = %d, want %d", response.StatusCode, fiber.StatusServiceUnavailable)
	}

	health, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer health.Body.Close()
	if health.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("health status = %d, want %d", health.StatusCode, fiber.StatusServiceUnavailable)
	}

	if err := state.importIndex(); err != nil {
		t.Fatalf("importIndex() error = %v", err)
	}
	ready, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer ready.Body.Close()
	if ready.StatusCode != http.StatusOK {
		t.Fatalf("health status after import = %d, want %d", ready.StatusCode, http.StatusOK)
	}
}

func TestEmbeddingEndpointRejectsHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<!doctype html><title>Temps</title>")
	}))
	defer server.Close()

	embed := newEmbeddingFunc(server.URL+"/v1", "", "test-model", server.Client())
	_, err := embed(context.Background(), "prueba")
	if err == nil {
		t.Fatal("expected HTML response to be rejected")
	}
	if !strings.Contains(err.Error(), `"text/html"`) || !strings.Contains(err.Error(), "Temps") {
		t.Fatalf("error does not identify the unexpected HTML response: %v", err)
	}
}

func createTestChromaDB(t *testing.T, path, content string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`CREATE TABLE embeddings (id INTEGER PRIMARY KEY, embedding_id TEXT)`,
		`CREATE TABLE embedding_metadata (id INTEGER, key TEXT, string_value TEXT)`,
		`INSERT INTO embeddings (id, embedding_id) VALUES (1, 'fragment-1')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO embedding_metadata (id, key, string_value) VALUES (1, 'chroma:document', ?)`, content); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateFragmentsAreNotSearchable(t *testing.T) {
	for _, junk := range []string{"OZ", "132", "Melpómene", "VAMPIRO LA MASCARADA EDICIÓN 20? ANIVERSARIO\n159", "e.... e Alma Compartida"} {
		if isSearchableFragment(junk) {
			t.Errorf("isSearchableFragment(%q) = true, want false", junk)
		}
	}
	prose := "Los Clanes Independientes a menudo funcionan como Sectas a pequeña escala, con objetivos concretos, aunque no sea siempre el caso."
	if !isSearchableFragment(prose) {
		t.Error("isSearchableFragment() rejected a full sentence from the manual")
	}
}

func TestPathSectionBodyFindsManualDescriptions(t *testing.T) {
	documents := []chromaDocument{
		{id: "1", content: "227\nSenda Verde\nLa Senda Verde lidia con la manipulación de la materia vegetal de cualquier tipo. Teóricamente se puede controlar cualquier cosa que sea más compleja que un alga microscópica."},
		{id: "2", content: "Encanto de las Llamas\nEsta Senda proporciona al taumaturgo la capacidad de conjurar llamas mágicas; al principio se trata de fuegos pequeños, pero los magos hábiles pueden crear grandes incendios."},
	}
	body, ok := pathSectionBody(documents, "Senda Verde")
	if !ok || !strings.Contains(body, "materia vegetal") {
		t.Fatalf("Senda Verde body not found: %q %v", body, ok)
	}
	body, ok = pathSectionBody(documents, "El Encanto de las Llamas")
	if !ok || !strings.Contains(body, "llamas mágicas") {
		t.Fatalf("leading article prevented the section lookup: %q %v", body, ok)
	}
}

func TestPathTopicWordsIgnoreManualJargon(t *testing.T) {
	if topics := pathTopicWords("¿Qué sendas taumatúrgicas pueden seguir los Tremere y qué hace cada una?"); len(topics) != 0 {
		t.Fatalf("catalog question produced topic words: %#v", topics)
	}
	topics := pathTopicWords("¿Existe una senda del agua y otra de las plantas para los Tremere?")
	joined := strings.ToLower(strings.Join(topics, " "))
	if !strings.Contains(joined, "agua") || !strings.Contains(joined, "plantas") {
		t.Fatalf("water/plant question lost its topics: %#v", topics)
	}
}

func TestThaumaturgyCatalogAndTopicIntentsAreExclusive(t *testing.T) {
	if !asksForThaumaturgyCatalog("¿Qué sendas taumatúrgicas pueden seguir los Tremere y qué hace cada una?") {
		t.Fatal("catalog intent not recognized")
	}
	if asksForThaumaturgyPathTopic("¿Qué sendas taumatúrgicas pueden seguir los Tremere y qué hace cada una?") {
		t.Fatal("catalog question must not be treated as a single-topic question")
	}
	if asksForThaumaturgyCatalog("¿Existe una senda del agua y otra de las plantas para los Tremere?") {
		t.Fatal("a water/plant question must not dump the whole catalog")
	}
	if !asksForThaumaturgyPathTopic("¿Existe una senda del agua y otra de las plantas para los Tremere?") {
		t.Fatal("water/plant question should be handled as a topic question")
	}
}

func TestExtractPoolAndDifficulty(t *testing.T) {
	text := "Credibilidad [Manipulación o Percepción + Subterfugio]: El Talento Subterfugio se usa con Manipulación a la hora de perpetrar un timo o con Percepción cuando se quiere detectar uno. Todas las partes implicadas hacen una tirada (normalmente a dificultad 7), sea para decir una mentira o para detectarla."
	pool, difficulty, ok := extractPoolAndDifficulty(text)
	if !ok || pool != "Percepción + Subterfugio" || difficulty != "7" {
		t.Fatalf("extractPoolAndDifficulty() = %q, %q, %v", pool, difficulty, ok)
	}
	if _, _, ok := extractPoolAndDifficulty("El jugador tira Inteligencia + Subterfugio (la dificultad es la Fuerza de Voluntad temporal del sujeto)."); ok {
		t.Fatal("a non numeric difficulty must not produce a deterministic pool")
	}
	pool, difficulty, ok = extractPoolAndDifficulty("El jugador tira Percepción + Empatía, dificultad 8.")
	if !ok || pool != "Percepción + Empatía" || difficulty != "8" {
		t.Fatalf("extractPoolAndDifficulty() = %q, %q, %v", pool, difficulty, ok)
	}
}
