package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	chromem "github.com/philippgille/chromem-go"
)

const (
	maxManualContext = 48000
	// Presupuesto por tipo de consulta: una duda concreta necesita pocos
	// pasajes; una lista necesita más, pero ni un volcado del manual.
	focusedContextLimit = 14000
	broadContextLimit   = 24000
)

// Estos términos sólo ayudan a buscar. No determinan una regla ni una respuesta.
var searchStopWords = map[string]bool{
	"que": true, "cual": true, "cuales": true, "como": true, "cuando": true,
	"donde": true, "quien": true, "por": true, "para": true, "con": true,
	"sin": true, "del": true, "los": true, "las": true, "una": true,
	"uno": true, "unos": true, "unas": true, "son": true, "hay": true,
	"tengo": true, "tiene": true, "tienen": true, "puedo": true, "puede": true,
	"pueden": true, "quiero": true, "necesito": true, "debo": true,
	"hacer": true, "hace": true, "hacen": true, "usar": true, "usa": true,
	"existe": true, "existen": true, "explica": true, "explicar": true,
	"describe": true, "detalla": true, "enumera": true, "lista": true,
	"todos": true, "todas": true, "cada": true, "otra": true, "otro": true,
	"tipos": true, "tipo": true, "ramas": true, "rama": true,
	"personaje": true, "jugador": true, "manual": true,
}

// Palabras que aparecen en casi cualquier página del juego: sirven para
// entender la pregunta, pero nunca para decidir que un pasaje la responde.
var genericSectionTerms = map[string]bool{
	"senda": true, "sendas": true, "disciplina": true, "disciplinas": true,
	"ritual": true, "rituales": true, "poder": true, "poderes": true,
	"magia": true, "magico": true, "mago": true, "magos": true, "hechiceria": true,
	"nivel": true, "niveles": true, "punto": true, "puntos": true,
	"clan": true, "clanes": true, "secta": true, "sector": true,
	"vampiro": true, "vampiros": true, "vasto": true, "vastos": true,
	"taumaturgia": true, "necromancia": true, "personaje": true, "personajes": true,
	"manual": true, "regla": true, "reglas": true, "tirada": true, "tiradas": true,
	"sistema": true, "dificultad": true, "exitos": true, "fracaso": true,
	"narrador": true, "jugador": true, "juego": true,
	"aptitudes": true, "rasgo": true, "rasgos": true, "tradicion": true,
	"tradiciones": true, "via": true, "vias": true, "tipo": true, "tipos": true,
	"rama": true, "ramas": true, "forma": true, "formas": true, "uso": true,
	"poderes y debilidades": true, "indice": true,
}

var (
	// El manual rotula cada Poder con el nivel que lo abre («Auspex 5 o más»,
	// «Taumaturgia 3 o más», «Nivel 2»). Se leen esas etiquetas, nunca se
	// deducen contando la posición de los Poderes.
	levelLabelPattern  = regexp.MustCompile(`(?i)^\s*([^\n]{2,60}?)\s*(\d)\s+o\s+m[aá]s\s*$`)
	plainLevelPattern  = regexp.MustCompile(`(?i)^\s*nivel\s+(\d)\s*$`)
	powerHeadingRegexp = regexp.MustCompile(`(?i)^\s*e(?:\.{1,3})?\s+([^\n]+)$`)
)

// Recoge los Poderes de la sección nombrada por la pregunta junto al nivel que
// el propio manual declara. Si un Poder no tiene etiqueta de nivel, se marca
// como no indicado en lugar de asignarle uno.
func sectionPowerIndex(question string, documents []chromaDocument) string {
	target := ""
	for _, document := range documents {
		for _, section := range documentSections(document.content) {
			name := normalizeWords(section)
			if len(name) >= 4 && strings.Contains(" "+normalizeWords(question)+" ", " "+name+" ") {
				target = name
			}
		}
	}
	if target == "" {
		return ""
	}
	var lines []string
	seen := make(map[string]bool)
	level := ""
	inSection := false
	for _, document := range documents {
		_, body, _ := strings.Cut(document.content, "\n")
		if inSection && normalizeWords(strings.Join(documentSections(document.content), " ")) != target {
			// La sección termina cuando empieza otra con otro nombre.
			inSection = false
		}
		if !inSection {
			matchesSection := false
			for _, section := range documentSections(document.content) {
				if normalizeWords(section) == target {
					matchesSection = true
					break
				}
			}
			if !matchesSection {
				continue
			}
			inSection = true
		}
		for _, raw := range strings.Split(body, "\n") {
			line := strings.TrimSpace(raw)
			if line == "" {
				continue
			}
			if match := levelLabelPattern.FindStringSubmatch(line); len(match) == 3 && strings.Contains(normalizeWords(match[1]), target) {
				level = match[2]
				continue
			}
			if match := plainLevelPattern.FindStringSubmatch(line); len(match) == 2 {
				level = match[1]
				continue
			}
			match := powerHeadingRegexp.FindStringSubmatch(line)
			if len(match) != 2 {
				continue
			}
			name := strings.TrimSpace(match[1])
			if normalizeWords(name) == "" || seen[normalizeWords(name)] {
				continue
			}
			seen[normalizeWords(name)] = true
			if level != "" {
				lines = append(lines, fmt.Sprintf("- Nivel %s: %s", level, name))
			} else {
				lines = append(lines, fmt.Sprintf("- Nivel no indicado en el pasaje: %s", name))
			}
			// El rótulo se aplica sólo al Poder que la sigue: si el
			//Rotulo no está en este pasaje, el nivel queda sin declarar.
			level = ""
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\nPoderes de " + target + " con el nivel que declara el manual (no son opciones disponibles, no completes la lista):\n" + strings.Join(lines, "\n") + "\n"
}

func searchTerms(question string) []string {
	var terms []string
	seen := make(map[string]bool)
	for _, term := range strings.Fields(normalizeWords(question)) {
		if len(term) < 3 || searchStopWords[term] || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func asksForBroadCoverage(question string) bool {
	text := " " + normalizeWords(question) + " "
	for _, phrase := range []string{
		" sendas ", " ramas ", " tipos ", " rituales ", " disciplinas ",
		" poderes ", " opciones ", " cuales ", " todos ", " todas ",
		" lista ", " enumera ", " compara ", " diferencias ", " cada ",
		" puntos ", " niveles ",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func documentChapter(content string) string {
	header, _, _ := strings.Cut(content, "\n")
	parts := strings.Split(header, " · ")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func documentSections(content string) []string {
	header, _, _ := strings.Cut(content, "\n")
	parts := strings.Split(header, " · ")
	if len(parts) < 3 {
		return nil
	}
	return parts[1 : len(parts)-1]
}

func hasNamedHeading(questionText, content string) bool {
	_, body, _ := strings.Cut(content, "\n")
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "e ") || strings.HasPrefix(line, "e.") {
			line = strings.TrimLeft(strings.TrimPrefix(line, "e"), ". ")
		}
		name := normalizeWords(line)
		if len(name) >= 4 && len(name) <= 100 && strings.Contains(questionText, " "+name+" ") {
			return true
		}
	}
	return false
}

// Combina relevancia semántica, coincidencias de términos poco frecuentes y
// secciones nombradas por el usuario. Las fuentes conservan su texto original:
// no se generan niveles ni reservas a partir de su posición en el documento.
func retrieveManualContext(question string, results []chromem.Result, documents []chromaDocument, positions map[string]int, minSimilarity float32) (string, []source) {
	terms := searchTerms(question)
	// Un término genérico no sirve para seleccionar un pasaje: "senda",
	// "disciplina" o "poder" aparecen en casi todo el manual.
	informative := make([]string, 0, len(terms))
	for _, term := range terms {
		if !genericSectionTerms[term] {
			informative = append(informative, term)
		}
	}
	if len(informative) == 0 {
		informative = terms
	}
	frequencies := make(map[string]int)
	texts := make([]string, len(documents))
	for i, document := range documents {
		texts[i] = " " + normalizeWords(document.content) + " "
		for _, term := range informative {
			if strings.Contains(texts[i], " "+term+" ") {
				frequencies[term]++
			}
		}
	}
	type candidate struct {
		position   int
		score      float64
		similarity float32
	}
	byPosition := make(map[int]candidate)
	for _, result := range results {
		position, ok := positions[result.ID]
		if !ok || result.Similarity < minSimilarity {
			continue
		}
		byPosition[position] = candidate{position: position, score: 2 + float64(result.Similarity), similarity: result.Similarity}
	}
	questionText := " " + normalizeWords(question) + " "
	broad := asksForBroadCoverage(question)
	for i, document := range documents {
		if !isSearchableFragment(document.content) {
			continue
		}
		var lexical, total float64
		hits := 0
		for _, term := range informative {
			weight := math.Log(1 + float64(len(documents))/float64(1+frequencies[term]))
			total += weight
			if strings.Contains(texts[i], " "+term+" ") {
				lexical += weight
				hits++
			}
		}
		sectionMatch := false
		for _, section := range documentSections(document.content) {
			name := normalizeWords(section)
			if len(name) >= 4 && strings.Contains(questionText, " "+name+" ") {
				sectionMatch = true
				break
			}
		}
		headingMatch := hasNamedHeading(questionText, document.content)
		if hits == 0 && !sectionMatch && !headingMatch {
			continue
		}
		// Una sola coincidencia genérica no debe desplazar fuentes semánticas.
		if hits < min(2, len(informative)) && !sectionMatch && !headingMatch {
			continue
		}
		item := byPosition[i]
		item.position = i
		if total > 0 {
			item.score += 3 * lexical / total
		}
		if sectionMatch {
			item.score += 1
		}
		if headingMatch {
			item.score += 6
		}
		if broad && strings.Contains(normalizeWords(strings.Join(documentSections(document.content), " ")), "indice") {
			item.score += 3
		}
		byPosition[i] = item
	}
	var ranked []candidate
	for _, item := range byPosition {
		ranked = append(ranked, item)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].position < ranked[j].position
		}
		return ranked[i].score > ranked[j].score
	})
	// Una respuesta pertenece a una sección del manual. El capítulo mejor
	// situado se favorece, pero los demás no se descartan de golpe: siguen
	// entrando por puntuación si el manual trata el tema en varios capítulos.
	chapterScores := make(map[string]float64)
	for _, item := range ranked {
		if chapter := documentChapter(documents[item.position].content); chapter != "" {
			chapterScores[chapter] += item.score
		}
	}
	dominantChapter, dominantScore := "", 0.0
	total := 0.0
	for chapter, score := range chapterScores {
		total += score
		if score > dominantScore {
			dominantChapter, dominantScore = chapter, score
		}
	}
	concentrated := total > 0 && dominantScore >= 0.55*total
	if concentrated {
		questionTextNormalized := " " + normalizeWords(question) + " "
		demoted := make([]candidate, 0, len(ranked))
		for _, item := range ranked {
			chapter := documentChapter(documents[item.position].content)
			isIndex := strings.Contains(normalizeWords(strings.Join(documentSections(documents[item.position].content), " ")), "indice")
			if chapter == dominantChapter || isIndex {
				demoted = append(demoted, item)
				continue
			}
			named := false
			for _, section := range documentSections(documents[item.position].content) {
				name := normalizeWords(section)
				if len(name) >= 4 && strings.Contains(questionTextNormalized, " "+name+" ") {
					named = true
					break
				}
			}
			if named {
				demoted = append(demoted, item)
				continue
			}
			item.score -= 6
			demoted = append(demoted, item)
		}
		sort.SliceStable(demoted, func(i, j int) bool {
			if demoted[i].score == demoted[j].score {
				return demoted[i].position < demoted[j].position
			}
			return demoted[i].score > demoted[j].score
		})
		ranked = demoted
	}
	anchorLimit := 6
	continuations := []int{1, 2}
	if broad {
		anchorLimit = 12
		continuations = []int{1, 2, 3}
	}
	if len(ranked) > anchorLimit {
		ranked = ranked[:anchorLimit]
	}
	budget := focusedContextLimit
	if broad {
		budget = broadContextLimit
	}
	var context strings.Builder
	var sources []source
	seen := make(map[int]bool)
	add := func(position int, similarity float32) {
		if position < 0 || position >= len(documents) || seen[position] || !isSearchableFragment(documents[position].content) {
			return
		}
		excerpt := strings.TrimSpace(documents[position].content)
		block := fmt.Sprintf("\n[%d]\n%s\n", len(sources)+1, excerpt)
		if context.Len()+len(block) > budget {
			return
		}
		seen[position] = true
		context.WriteString(block)
		sources = append(sources, source{Excerpt: excerpt, Similarity: similarity})
	}
	// Cada coincidencia va acompañada de su continuación antes de pasar a la
	// siguiente: una regla y sus excepciones deben leerse juntas.
	for _, item := range ranked {
		add(item.position, item.similarity)
		for _, offset := range continuations {
			neighbor := item.position + offset
			if neighbor < 0 || neighbor >= len(documents) {
				continue
			}
			chapter := documentChapter(documents[item.position].content)
			if chapter != "" && documentChapter(documents[neighbor].content) != chapter {
				continue
			}
			add(neighbor, 0)
		}
	}
	manualContext := context.String()
	if index := sectionPowerIndex(question, documents); index != "" {
		manualContext = strings.TrimSpace(index) + "\n" + manualContext
	}
	return manualContext, sources
}
