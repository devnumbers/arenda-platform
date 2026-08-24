-- Down restores the full schema of the dropped tables as of migration 113
-- (reconstructed with pg_dump from the migrated state), without data: the
-- down chain below 114 still ALTERs these tables (e.g. 000109 leases end-date
-- invariant), so the tables must exist for the roll-back to reach zero.
-- Dead enum values (notification_event_type / notification_target_type) stay
-- in the database (precedent #277); audit history is not touched.
--
-- Name: leases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE leases (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    property_id uuid,
    tenant_contact_id uuid,
    status text NOT NULL,
    start_date date NOT NULL,
    end_date date,
    rent_amount_kopecks bigint NOT NULL,
    deposit_amount_kopecks bigint DEFAULT 0 NOT NULL,
    payment_day integer NOT NULL,
    comment text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_leases_end_not_before_start CHECK (((end_date IS NULL) OR (end_date >= start_date))),
    CONSTRAINT leases_deposit_amount_kopecks_check CHECK ((deposit_amount_kopecks >= 0)),
    CONSTRAINT leases_payment_day_check CHECK (((payment_day >= 1) AND (payment_day <= 31))),
    CONSTRAINT leases_rent_amount_kopecks_check CHECK ((rent_amount_kopecks >= 0)),
    CONSTRAINT leases_status_check CHECK ((status = ANY (ARRAY['awaiting_start'::text, 'active'::text, 'requires_action'::text, 'completed'::text, 'archived'::text])))
);


--
-- Name: operation_categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE operation_categories (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    type text NOT NULL,
    name text NOT NULL,
    code text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT operation_categories_type_check CHECK ((type = ANY (ARRAY['income'::text, 'expense'::text])))
);


--
-- Name: operations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE operations (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    property_id uuid,
    lease_id uuid,
    recurring_operation_id uuid,
    type text NOT NULL,
    amount_kopecks bigint NOT NULL,
    operation_date date NOT NULL,
    comment text,
    is_exception boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    status text DEFAULT 'pending'::text NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    reminder_offset_days integer,
    source_operation_date date,
    category_id uuid NOT NULL,
    CONSTRAINT operations_amount_kopecks_check CHECK ((amount_kopecks >= 0)),
    CONSTRAINT operations_reminder_offset_days_check CHECK ((reminder_offset_days = ANY (ARRAY[1, 3, 7]))),
    CONSTRAINT operations_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'overdue'::text, 'paid'::text, 'received'::text, 'unconfirmed'::text]))),
    CONSTRAINT operations_type_check CHECK ((type = ANY (ARRAY['income'::text, 'expense'::text])))
);


--
-- Name: recurring_operations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE recurring_operations (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    property_id uuid,
    lease_id uuid,
    type text NOT NULL,
    amount_kopecks bigint NOT NULL,
    start_date date NOT NULL,
    payment_day integer NOT NULL,
    end_date date,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    periodicity text DEFAULT 'monthly'::text NOT NULL,
    comment text,
    status text DEFAULT 'active'::text NOT NULL,
    reminder_offset_days integer,
    name text DEFAULT ''::text NOT NULL,
    deleted_at timestamp with time zone,
    category_id uuid NOT NULL,
    CONSTRAINT chk_recurring_offset_nonnegative CHECK ((reminder_offset_days >= 0)),
    CONSTRAINT chk_recurring_operations_payment_day CHECK (((payment_day >= 1) AND (payment_day <= 31))),
    CONSTRAINT recurring_operations_amount_kopecks_check CHECK ((amount_kopecks >= 0)),
    CONSTRAINT recurring_operations_periodicity_check CHECK ((periodicity = 'monthly'::text)),
    CONSTRAINT recurring_operations_reminder_offset_days_check CHECK ((reminder_offset_days = ANY (ARRAY[1, 3, 7]))),
    CONSTRAINT recurring_operations_status_check CHECK ((status = ANY (ARRAY['active'::text, 'paused'::text]))),
    CONSTRAINT recurring_operations_type_check CHECK ((type = ANY (ARRAY['income'::text, 'expense'::text])))
);


--
-- Name: reminders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE reminders (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    target_type notification_target_type NOT NULL,
    operation_id uuid,
    recurring_operation_id uuid,
    lease_id uuid,
    property_id uuid,
    event_type notification_event_type NOT NULL,
    status notification_status DEFAULT 'pending'::notification_status NOT NULL,
    scheduled_at timestamp with time zone NOT NULL,
    sent_at timestamp with time zone,
    failed_attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone,
    message_title text NOT NULL,
    message_body text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT exactly_one_target CHECK ((((target_type = 'operation'::notification_target_type) AND (operation_id IS NOT NULL) AND (lease_id IS NULL)) OR ((target_type = 'recurring_operation'::notification_target_type) AND (recurring_operation_id IS NOT NULL) AND (operation_id IS NULL) AND (lease_id IS NULL)) OR ((target_type = 'lease'::notification_target_type) AND (lease_id IS NOT NULL) AND (operation_id IS NULL))))
);


--
-- Name: sent_email_reminders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sent_email_reminders (
    id uuid NOT NULL,
    reminder_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    email text NOT NULL,
    subject text NOT NULL,
    plain_body text NOT NULL,
    sent_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: sent_push_reminders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sent_push_reminders (
    id uuid NOT NULL,
    reminder_id uuid NOT NULL,
    recipient_id uuid NOT NULL,
    sent_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenant_contacts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE tenant_contacts (
    id uuid NOT NULL,
    owner_id uuid NOT NULL,
    name text NOT NULL,
    surname text,
    patronymic text,
    phone text,
    email text,
    comment text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: leases leases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY leases
    ADD CONSTRAINT leases_pkey PRIMARY KEY (id);


--
-- Name: operation_categories operation_categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operation_categories
    ADD CONSTRAINT operation_categories_pkey PRIMARY KEY (id);


--
-- Name: operations operations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operations
    ADD CONSTRAINT operations_pkey PRIMARY KEY (id);


--
-- Name: recurring_operations recurring_operations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recurring_operations
    ADD CONSTRAINT recurring_operations_pkey PRIMARY KEY (id);


--
-- Name: reminders reminders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY reminders
    ADD CONSTRAINT reminders_pkey PRIMARY KEY (id);


--
-- Name: sent_email_reminders sent_email_reminders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sent_email_reminders
    ADD CONSTRAINT sent_email_reminders_pkey PRIMARY KEY (id);


--
-- Name: sent_push_reminders sent_push_reminders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sent_push_reminders
    ADD CONSTRAINT sent_push_reminders_pkey PRIMARY KEY (id);


--
-- Name: tenant_contacts tenant_contacts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tenant_contacts
    ADD CONSTRAINT tenant_contacts_pkey PRIMARY KEY (id);


--
-- Name: sent_email_reminders uq_sent_email_reminders_reminder_recipient; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sent_email_reminders
    ADD CONSTRAINT uq_sent_email_reminders_reminder_recipient UNIQUE (reminder_id, owner_id);


--
-- Name: idx_leases_one_open_per_property; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_leases_one_open_per_property ON leases USING btree (property_id) WHERE (status = ANY (ARRAY['awaiting_start'::text, 'active'::text, 'requires_action'::text]));


--
-- Name: idx_leases_open_past_end; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_leases_open_past_end ON leases USING btree (end_date) WHERE ((status = ANY (ARRAY['awaiting_start'::text, 'active'::text, 'requires_action'::text])) AND (end_date IS NOT NULL));


--
-- Name: idx_leases_owner_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_leases_owner_status ON leases USING btree (owner_id, status);


--
-- Name: idx_leases_owner_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_leases_owner_updated_at ON leases USING btree (owner_id, updated_at DESC);


--
-- Name: idx_leases_property_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_leases_property_status ON leases USING btree (property_id, status);


--
-- Name: idx_operation_categories_owner_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_operation_categories_owner_code ON operation_categories USING btree (owner_id, code) WHERE (code IS NOT NULL);


--
-- Name: idx_operation_categories_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operation_categories_owner_id ON operation_categories USING btree (owner_id);


--
-- Name: idx_operation_categories_owner_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operation_categories_owner_type ON operation_categories USING btree (owner_id, type);


--
-- Name: idx_operation_categories_owner_type_lower_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_operation_categories_owner_type_lower_name ON operation_categories USING btree (owner_id, type, lower(name));


--
-- Name: idx_operations_lease_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_lease_date ON operations USING btree (lease_id, operation_date);


--
-- Name: idx_operations_lease_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_lease_not_deleted ON operations USING btree (lease_id, operation_date) WHERE (deleted_at IS NULL);


--
-- Name: idx_operations_owner_operation_date_id_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_owner_operation_date_id_not_deleted ON operations USING btree (owner_id, operation_date DESC, id DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_operations_owner_pending_operation_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_owner_pending_operation_date ON operations USING btree (owner_id, operation_date) WHERE ((deleted_at IS NULL) AND (status = 'pending'::text));


--
-- Name: idx_operations_property_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_property_date ON operations USING btree (property_id, operation_date);


--
-- Name: idx_operations_property_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_property_not_deleted ON operations USING btree (property_id, operation_date) WHERE (deleted_at IS NULL);


--
-- Name: idx_operations_recurring_date_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_operations_recurring_date_unique ON operations USING btree (recurring_operation_id, operation_date) WHERE ((recurring_operation_id IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: idx_operations_recurring_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_recurring_not_deleted ON operations USING btree (recurring_operation_id, operation_date) WHERE (deleted_at IS NULL);


--
-- Name: idx_operations_reminder_offset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_operations_reminder_offset ON operations USING btree (reminder_offset_days) WHERE (reminder_offset_days IS NOT NULL);


--
-- Name: idx_recurring_operations_owner_not_deleted; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_recurring_operations_owner_not_deleted ON recurring_operations USING btree (owner_id, created_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_reminders_active_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_reminders_active_unique ON reminders USING btree (owner_id, target_type, operation_id, recurring_operation_id, lease_id, event_type) WHERE (status = ANY (ARRAY['pending'::notification_status, 'sending'::notification_status]));


--
-- Name: idx_reminders_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminders_due ON reminders USING btree (scheduled_at, next_attempt_at) WHERE (status = 'pending'::notification_status);


--
-- Name: idx_reminders_lease; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminders_lease ON reminders USING btree (lease_id);


--
-- Name: idx_reminders_operation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminders_operation ON reminders USING btree (operation_id);


--
-- Name: idx_reminders_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminders_owner ON reminders USING btree (owner_id, status, scheduled_at);


--
-- Name: idx_reminders_property_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminders_property_pending ON reminders USING btree (property_id, scheduled_at) WHERE ((target_type = 'free'::notification_target_type) AND (status = 'pending'::notification_status));


--
-- Name: idx_reminders_recurring; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminders_recurring ON reminders USING btree (recurring_operation_id);


--
-- Name: idx_sent_email_reminders_owner_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sent_email_reminders_owner_id ON sent_email_reminders USING btree (owner_id);


--
-- Name: idx_sent_email_reminders_sent_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sent_email_reminders_sent_at ON sent_email_reminders USING btree (sent_at);


--
-- Name: idx_sent_push_reminders_recipient; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sent_push_reminders_recipient ON sent_push_reminders USING btree (recipient_id);


--
-- Name: idx_sent_push_reminders_sent_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sent_push_reminders_sent_at ON sent_push_reminders USING btree (sent_at);


--
-- Name: idx_tenant_contacts_owner_phone; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_tenant_contacts_owner_phone ON tenant_contacts USING btree (owner_id, phone) WHERE (phone IS NOT NULL);


--
-- Name: uq_sent_push_reminders_reminder_recipient; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_sent_push_reminders_reminder_recipient ON sent_push_reminders USING btree (reminder_id, recipient_id);


--
-- Name: leases trg_leases_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_leases_updated_at BEFORE UPDATE ON leases FOR EACH ROW EXECUTE FUNCTION set_updated_at();


--
-- Name: operation_categories trg_operation_categories_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_operation_categories_updated_at BEFORE UPDATE ON operation_categories FOR EACH ROW EXECUTE FUNCTION set_updated_at();


--
-- Name: operations trg_operations_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_operations_updated_at BEFORE UPDATE ON operations FOR EACH ROW EXECUTE FUNCTION set_updated_at();


--
-- Name: recurring_operations trg_recurring_operations_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_recurring_operations_updated_at BEFORE UPDATE ON recurring_operations FOR EACH ROW EXECUTE FUNCTION set_updated_at();


--
-- Name: reminders trg_reminders_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_reminders_updated_at BEFORE UPDATE ON reminders FOR EACH ROW EXECUTE FUNCTION set_updated_at();


--
-- Name: tenant_contacts trg_tenant_contacts_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_tenant_contacts_updated_at BEFORE UPDATE ON tenant_contacts FOR EACH ROW EXECUTE FUNCTION set_updated_at();


--
-- Name: leases leases_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY leases
    ADD CONSTRAINT leases_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: leases leases_property_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY leases
    ADD CONSTRAINT leases_property_id_fkey FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;


--
-- Name: leases leases_tenant_contact_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY leases
    ADD CONSTRAINT leases_tenant_contact_id_fkey FOREIGN KEY (tenant_contact_id) REFERENCES tenant_contacts(id) ON DELETE SET NULL;


--
-- Name: operation_categories operation_categories_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operation_categories
    ADD CONSTRAINT operation_categories_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: operations operations_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operations
    ADD CONSTRAINT operations_category_id_fkey FOREIGN KEY (category_id) REFERENCES operation_categories(id) ON DELETE RESTRICT;


--
-- Name: operations operations_lease_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operations
    ADD CONSTRAINT operations_lease_id_fkey FOREIGN KEY (lease_id) REFERENCES leases(id) ON DELETE CASCADE;


--
-- Name: operations operations_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operations
    ADD CONSTRAINT operations_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: operations operations_property_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operations
    ADD CONSTRAINT operations_property_id_fkey FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;


--
-- Name: operations operations_recurring_operation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY operations
    ADD CONSTRAINT operations_recurring_operation_id_fkey FOREIGN KEY (recurring_operation_id) REFERENCES recurring_operations(id) ON DELETE CASCADE;


--
-- Name: recurring_operations recurring_operations_category_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recurring_operations
    ADD CONSTRAINT recurring_operations_category_id_fkey FOREIGN KEY (category_id) REFERENCES operation_categories(id) ON DELETE RESTRICT;


--
-- Name: recurring_operations recurring_operations_lease_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recurring_operations
    ADD CONSTRAINT recurring_operations_lease_id_fkey FOREIGN KEY (lease_id) REFERENCES leases(id) ON DELETE CASCADE;


--
-- Name: recurring_operations recurring_operations_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recurring_operations
    ADD CONSTRAINT recurring_operations_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: recurring_operations recurring_operations_property_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY recurring_operations
    ADD CONSTRAINT recurring_operations_property_id_fkey FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;


--
-- Name: reminders reminders_lease_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY reminders
    ADD CONSTRAINT reminders_lease_id_fkey FOREIGN KEY (lease_id) REFERENCES leases(id) ON DELETE CASCADE;


--
-- Name: reminders reminders_operation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY reminders
    ADD CONSTRAINT reminders_operation_id_fkey FOREIGN KEY (operation_id) REFERENCES operations(id) ON DELETE CASCADE;


--
-- Name: reminders reminders_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY reminders
    ADD CONSTRAINT reminders_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: reminders reminders_property_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY reminders
    ADD CONSTRAINT reminders_property_id_fkey FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;


--
-- Name: reminders reminders_recurring_operation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY reminders
    ADD CONSTRAINT reminders_recurring_operation_id_fkey FOREIGN KEY (recurring_operation_id) REFERENCES recurring_operations(id) ON DELETE CASCADE;


--
-- Name: sent_email_reminders sent_email_reminders_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sent_email_reminders
    ADD CONSTRAINT sent_email_reminders_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: sent_email_reminders sent_email_reminders_reminder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sent_email_reminders
    ADD CONSTRAINT sent_email_reminders_reminder_id_fkey FOREIGN KEY (reminder_id) REFERENCES reminders(id) ON DELETE CASCADE;


--
-- Name: sent_push_reminders sent_push_reminders_recipient_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sent_push_reminders
    ADD CONSTRAINT sent_push_reminders_recipient_id_fkey FOREIGN KEY (recipient_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: sent_push_reminders sent_push_reminders_reminder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sent_push_reminders
    ADD CONSTRAINT sent_push_reminders_reminder_id_fkey FOREIGN KEY (reminder_id) REFERENCES reminders(id) ON DELETE CASCADE;


--
-- Name: tenant_contacts tenant_contacts_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY tenant_contacts
    ADD CONSTRAINT tenant_contacts_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
