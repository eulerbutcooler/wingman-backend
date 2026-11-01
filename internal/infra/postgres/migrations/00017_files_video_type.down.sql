ALTER TABLE files DROP CONSTRAINT IF EXISTS files_file_type_check;
ALTER TABLE files
    ADD CONSTRAINT files_file_type_check
    CHECK (file_type IN ('pdf', 'ppt', 'docx'));
