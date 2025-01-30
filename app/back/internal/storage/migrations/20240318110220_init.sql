-- +goose Up
-- +goose StatementBegin

create table public.test_table(
                                  id         BIGSERIAL Primary KEY,
                                  created_at TIMESTAMP NOT NULL default CURRENT_TIMESTAMP,
                                  deleted_at TIMESTAMP NULL,
                                  name        varchar(50) NOT NULL check( trim(name) <> '' ),
                                  unique (name)
);

insert into public.test_table (name)
values ('Антон'), ('Андрей'), ('Карина'), ('Игорь');


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table public."test_table";
-- +goose StatementEnd

