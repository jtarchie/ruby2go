-- Seed data, generated from the app's own db/seeds.rb.
-- The archive is text-only, so no binary database ships; this file
-- populates a fresh one for any target: `sqlite3 <db> < db/seed.sql`.
-- INSERTs name their columns explicitly so the file stays valid whatever
-- order a target emits its schema columns in.
-- created_at/updated_at are synthesized (seeds.rb sets neither, and the
-- columns are NOT NULL): a fixed base instant plus one microsecond per
-- row, in creation order, so ordering by them is deterministic.
CREATE TABLE IF NOT EXISTS articles ( id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT, body TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS comments ( id INTEGER PRIMARY KEY AUTOINCREMENT, article_id INTEGER NOT NULL, commenter TEXT, body TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS index_comments_on_article_id ON comments (article_id);
INSERT INTO articles (id, title, body, created_at, updated_at) VALUES (1,'Getting Started with Rails','Rails is a web application framework running on the Ruby programming language. It makes building web apps faster and easier with conventions over configuration.','2026-01-01 00:00:00.000001','2026-01-01 00:00:00.000001');
INSERT INTO comments (id, article_id, commenter, body, created_at, updated_at) VALUES (1,1,'Alice','Great introduction! Rails really does make development faster.','2026-01-01 00:00:00.000002','2026-01-01 00:00:00.000002');
INSERT INTO comments (id, article_id, commenter, body, created_at, updated_at) VALUES (2,1,'Bob','I love how Rails handles database migrations automatically.','2026-01-01 00:00:00.000003','2026-01-01 00:00:00.000003');
INSERT INTO articles (id, title, body, created_at, updated_at) VALUES (2,'Understanding MVC Architecture','MVC stands for Model-View-Controller. Models handle data and business logic, Views display information to users, and Controllers coordinate between them.','2026-01-01 00:00:00.000004','2026-01-01 00:00:00.000004');
INSERT INTO comments (id, article_id, commenter, body, created_at, updated_at) VALUES (3,2,'Carol','This pattern really helps keep code organized!','2026-01-01 00:00:00.000005','2026-01-01 00:00:00.000005');
INSERT INTO articles (id, title, body, created_at, updated_at) VALUES (3,'Ruby2JS: Rails Everywhere','Ruby2JS transpiles Ruby to JavaScript, enabling Rails applications to run in browsers, on Node.js, and at the edge. Same code, different runtimes.','2026-01-01 00:00:00.000006','2026-01-01 00:00:00.000006');
