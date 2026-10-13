# Codex · El saber de la noche

Buscador RAG para **Vampiro: La Mascarada V20**, con backend Go (Fiber v3 y chromem-go) e interfaz web gótica. Este repositorio es el despliegue: el servidor, el front y los datos.

## Ejecutar

Requiere Go 1.26 o superior y que los endpoints configurados de chat y embeddings respondan.

```sh
cp .env.example .env        # ajusta modelos, endpoints y claves
set -a; source .env; set +a
go build -o codex .
./codex
```

El frontend está embebido en el ejecutable y se sirve desde la raíz. Cuando el backend inicia correctamente, abre **http://localhost:3000**. El puerto se puede cambiar con `PORT`.

El servidor levanta de inmediato y la indexación corre en segundo plano: `/api/health` responde `503 indexing` y `/api/search` responde `503` hasta que el índice esté listo. En el primer arranque se importan los textos de ChromaDB y se generan sus embeddings en `codex_data/chromem`. Se omiten los fragmentos sin contenido útil (encabezados sueltos, números de página), que solo ensucian la búsqueda. Esta importación puede tardar varios minutos; si se interrumpe, al volver a iniciar continúa con los fragmentos faltantes. Las siguientes ejecuciones reutilizan esa colección.

## Docker

El `Dockerfile` multi-stage compila el binario y hornea los datos: `v20_db/chroma.sqlite3` + el índice precocido `codex_data/chromem.tar.gz` (los 1860 fragmentos ya embebidos), así el contenedor arranca listo, sin importación fría.

```sh
docker build -t codex .
docker run -p 3000:3000 \
  -e LLM_BASE_URL=http://192.168.100.100:8080/v1 \
  -e EMBEDDING_BASE_URL=http://192.168.100.100:8080/v1 \
  codex
```

Si cambiás `EMBEDDING_MODEL`, borrá `CHROMEM_DB_PATH` dentro del contenedor (o montá un volumen vacío en esa ruta) para que regenere la colección local; el índice horneado solo vale para el modelo con el que se creó.

## Los datos

`v20_db/chroma.sqlite3` es una ChromaDB persistente con el manual troceado en **1860 fragmentos** (mediana de ~1200 caracteres, cortados por párrafos, con tablas y listas intactas y una cabecera `Capítulo · Sección · pág. A-B`). Los vectores son de 1024 dimensiones y el texto de cada fragmento vive en el metadato `chroma:document`, que la app lee ordenado por identificador para reconstruir el orden del libro.

Los vectores se calculan con el modelo indicado en `EMBEDDING_MODEL`: usa el mismo para crear y consultar. Si cambias el modelo, asigna otra ruta mediante `CHROMEM_DB_PATH` para regenerar la colección local.

`codex_data/chromem.tar.gz` es ese índice local empaquetado (1860 vectores listos). El repositorio lo versiona para que el `Dockerfile` lo hornee en la imagen; en uso normal podés ignorarlo o borrarlo y la app lo regenera.

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

El endpoint de embeddings debe ser compatible con OpenAI y servir el modelo con el preset `embedding=true` y `pooling=last`.

## API de Codex

- `GET /api/health` — estado y número de fragmentos indexados (`503` + `"indexing"` mientras prepara el índice).
- `POST /api/search` — recibe `{ "question": "..." }` y devuelve la respuesta y sus fragmentos fuente (`503` mientras el índice se prepara).
