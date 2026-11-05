INSERT INTO tasks (title, description) VALUES
  ('Setup Project', 'Initialize Go module and database connection'),
  ('Implement Auth', 'Add JWT user authentication'),
  ('Deploy App', 'Set up CI/CD pipeline')
ON CONFLICT DO NOTHING;