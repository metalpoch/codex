# Codex · El saber de la noche

Buscador RAG para **Vampiro: La Mascarada V20**, con backend Go (Fiber v3 y chromem-go) e interfaz web gótica.

## Ejecutar

Requiere Go 1.26 o superior, `v20_db/chroma.sqlite3` y que los endpoints configurados de chat y embeddings respondan.

```sh
go build -o codex .
./codex
```

El frontend está embebido en el ejecutable y se sirve desde la raíz. Cuando el backend inicia correctamente, abre **http://localhost:3000**. El puerto se puede cambiar con `PORT`.

En el primer arranque se importan los textos de ChromaDB y se generan sus embeddings en `codex_data/chromem`. Se omiten los fragmentos sin contenido útil (encabezados sueltos, números de página), que solo ensucian la búsqueda. Esta importación puede tardar; si se interrumpe, al volver a iniciar continúa con los fragmentos faltantes. Las siguientes ejecuciones reutilizan esa colección.

## Cómo responde Codex

Las tiradas, las listas de Poderes por puntuación y los catálogos de Sendas se resuelven leyendo el manual directamente, sin dejar que el modelo rellene huecos. Cuando un dato no aparece en los fragmentos, Codex lo dice en lugar de inventarlo.

## Configuración de Codex

| Variable | Predeterminado | Uso |
| --- | --- | --- |
| `PORT` | `3000` | Puerto de la web |
| `CHROMA_DB_PATH` | `./v20_db/chroma.sqlite3` | Base ChromaDB de origen |
| `CHROMEM_DB_PATH` | `./codex_data/chromem` | Colección local de chromem-go |
| `LLM_BASE_URL` | `http://192.168.100.100:8080/v1` | Endpoint de chat |
| `LLM_MODEL` | `mradermacher/Qwen-3-VL-8B-Instruct-heretic-GGUF:Q4_K_M` | Modelo de respuesta |
| `EMBEDDING_BASE_URL` | `http://192.168.100.100:8080/v1` | Endpoint de embeddings |
| `EMBEDDING_MODEL` | `Qwen/Qwen3-Embedding-0.6B-GGUF:F16` | Modelo de embeddings |
| `LLM_API_KEY` | `no-key-required` | Clave opcional de chat |
| `EMBEDDING_API_KEY` | valor de `LLM_API_KEY` | Clave opcional de embeddings |

Usa el mismo modelo de embeddings para crear y consultar vectores. Si cambias el modelo, asigna otra ruta mediante `CHROMEM_DB_PATH` para regenerar la colección.

## API de Codex

- `GET /api/health` — estado y número de fragmentos indexados.
- `POST /api/search` — recibe `{ "question": "..." }` y devuelve la respuesta y sus fragmentos fuente.
