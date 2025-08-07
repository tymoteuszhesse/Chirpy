- generate queries:
`sqlc generate`
- migrations:
`cd sql/schema`
`goose postgres "postgres://tymoteuszhesse:@localhost:5432/chirpy" up`

psql postgres 
\c chirpy

lub 
psql chirpy
\dt

