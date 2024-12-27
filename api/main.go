package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

type Cancion struct {
	ID     int    `json:"id_canciones"`
	Nombre string `json:"nombre"`
}

type Letra struct {
	Texto string `json:"texto"`
	IDEstructura string `json:"id_estructura_canciones"`
}

type Parrafo struct {
	IDEstructura string `json:"id_estructura_canciones"`
	Descripcion string `json:"descripcion_parrafo"`
}

var db *sql.DB

func main() {
	// Conexión a la base de datos
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error al abrir la conexión: %v", err)
	}
	defer db.Close()

	// Verificar conexión
	if err = db.Ping(); err != nil {
		log.Fatalf("Error al conectar a la base de datos: %v", err)
	}

	// Rutas HTTP
	http.HandleFunc("/lista_canciones", listaCancionesHandler)
	http.HandleFunc("/consulta_letra_cancion", consultaLetraCancionHandler)
	http.HandleFunc("/consulta_parrafo_cancion", consultaParrafoCancionHandler)
	http.HandleFunc("/consulta_letra_cifrado_cancion", consultaLetraCifradoCancionHandler)


	fmt.Println("Servidor escuchando en el puerto 8080...22:55")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func listaCancionesHandler(w http.ResponseWriter, r *http.Request) {
	canciones, err := listaCanciones()
	if err != nil {
		http.Error(w, "Error al obtener la lista de canciones", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(canciones)
}

func consultaLetraCancionHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Falta el parámetro 'id'", http.StatusBadRequest)
		return
	}

	letra, err := consultaLetraCancion(id)
	if err != nil {
		http.Error(w, "Error al obtener la letra de la canción", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(letra)
}

func consultaParrafoCancionHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Falta el parámetro 'id'", http.StatusBadRequest)
		return
	}

	parrafo, err := consultaParrafoCancion(id)
	if err != nil {
		http.Error(w, "Error al obtener la letra de la canción", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(parrafo)
}


func consultaLetraCifradoCancionHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	tono := r.URL.Query().Get("tono")

	if id == "" || tono == "" {
		http.Error(w, "Faltan los parámetros 'id' y/o 'tono'", http.StatusBadRequest)
		return
	}

	resultados, err := consultaLetraCifradoCancion(id, tono)
	if err != nil {
		http.Error(w, "Error al obtener la letra cifrada de la canción", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(resultados)
}



func listaCanciones() ([]Cancion, error) {
	rows, err := db.Query("SELECT id_canciones, nombre FROM canciones ORDER BY nombre")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var canciones []Cancion
	for rows.Next() {
		var c Cancion
		if err := rows.Scan(&c.ID, &c.Nombre); err != nil {
			return nil, err
		}
		canciones = append(canciones, c)
	}	
	return canciones, nil
}

func consultaLetraCancion(id string) ([]Letra, error) {
	rows, err := db.Query("SELECT id_estructura_canciones,texto FROM lineas_canciones WHERE id_canciones = ?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letra []Letra
	for rows.Next() {
		var l Letra
		if err := rows.Scan(&l.IDEstructura,&l.Texto); err != nil {
			return nil, err
		}

		letra = append(letra, l)
	}
	return letra, nil
}

func consultaParrafoCancion(id string) ([]Parrafo, error) {
	rows, err := db.Query("SELECT ec.id_estructura_canciones, concat(tl.descripcion,cast(ec.tipo_linea_numero as char(50))) as descripcion_parrafo FROM tipos_linea tl LEFT JOIN estructura_canciones ec ON ec.id_tipo_linea = tl.id WHERE ec.id_canciones = ? order by ec.posicion_estructura", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parrafo []Parrafo
	for rows.Next() {
		var p Parrafo
		if err := rows.Scan(&p.IDEstructura,&p.Descripcion,); err != nil {
			return nil, err
		}

		parrafo = append(parrafo, p)
	}
	return parrafo, nil
}

func consultaLetraCifradoCancion(id string, tono string) ([]map[string]string, error) {
	query := `
		SELECT 
			CONCAT(
				(SELECT tonalidades.codigo FROM tonalidades WHERE tonalidades.grado = acordes_linea.grado + ?),
				IFNULL(triadas.triadas, ''),
				IFNULL(extensiones.extensiones, '')
			) AS acorde,
			SUBSTR(
				lineas_canciones.texto,
				acordes_linea.ubicacion,
				IF(
					acordes_linea.id_lineas_canciones = LEAD(acordes_linea.id_lineas_canciones) OVER (ORDER BY acordes_linea.id_acordes_linea),
					LEAD(acordes_linea.ubicacion) OVER (ORDER BY acordes_linea.id_acordes_linea) - 1,
					LENGTH(lineas_canciones.texto)
				)
			) AS letra
		FROM 
			lineas_canciones
		LEFT JOIN 
			acordes_linea ON acordes_linea.id_lineas_canciones = lineas_canciones.id_lineas_canciones
		LEFT JOIN 
			triadas ON acordes_linea.id_triadas = triadas.id_triadas
		LEFT JOIN 
			extensiones ON acordes_linea.id_extensiones = extensiones.id_extensiones
		WHERE 
			lineas_canciones.id_canciones = ?
	`

	rows, err := db.Query(query, tono, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resultados []map[string]string
	for rows.Next() {
		var acorde, letra string
		if err := rows.Scan(&acorde, &letra); err != nil {
			return nil, err
		}
		resultados = append(resultados, map[string]string{
			"acorde": acorde,
			"letra":  letra,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return resultados, nil
}
