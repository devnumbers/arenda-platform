'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { notify } from '@/shared/lib/notifications';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { readPhotoDataUrl } from '@/shared/lib/photo';
import type { AttrErrors, AttrKey } from '@/features/property-attributes';
import {
  buildPropertyEditCommand,
  initialPropertyEditDraft,
  propertyAutonamePhrase,
  propertyEditDirty,
  type PropertyEditDraft,
  type PropertyPhotoStage,
  useDeletePropertyPhoto,
  useProperty,
  useUpdateProperty,
  useUploadPropertyPhoto,
} from '@/features/properties';
import { attributeCatalog } from '../property-fields/attribute-catalog';
import { propertyPermissions, type PropertyType } from '@/entities/property';
import { ApiError } from '@/shared/api/errors';
import {
  Button,
  IconButton,
  PageContent,
  Skeleton,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavTitle,
  useTabBarSuppression,
} from '@/shared/ui/design';
import { Cancel, Check } from '@/shared/assets/icons';
import { PropertyCatalogFields } from '../property-fields/property-catalog-fields';
import { PropertyHousingTypeChips } from '../property-fields/property-housing-type-chips';
import { PropertyAddressSearch } from './property-address-search';
import { PropertyPhotoSlot } from './property-photo-slot';
import { PropertyTypePicker } from './property-type-picker';

/**
 * Экран «Редактирование объекта» (карта #583, тикет #590; Figma
 * 1550:95852) — страница-маршрут /properties/[id]/edit, замена старой
 * формы на едином хроме подэкрана. Хедер: крестик слева (закрыть),
 * заголовок по центру, галочка справа (сохранить — тот же сабмит, что и
 * StickyBottomBar «Сохранить изменения»). Поля: фото-слот (глиф по типу
 * либо загруженное фото; изменение фото — часть черновика: stage
 * (PropertyPhotoStage) применяется кнопками сохранения вместе с полями —
 * PATCH свойств и фото-эндпоинты (ADR 0065, тикет #1228) уходят
 * параллельно, уход без сохранения сервер не трогает; решение владельца
 * 08.10),
 * тип (PickerField
 * с шитом чипов 1554:97471), адрес
 * (тап — полноэкранный поиск адреса с подсказками DaData, 1518:93118/
 * 93341), название 0/64, «Тип жилья» и поля каталога — общая часть с
 * визардом создания (property-fields), описание 0/1024.
 *
 * Смена типа lossless (как в визарде): чужие ключи прежнего типа
 * хранятся в черновике, в payload идут только ключи каталога нового
 * типа. Готовность — обязательные поля + валидный каталог; кнопки
 * сохранения погашены, пока правки нет — полей (propertyEditDirty) или
 * фото (photoStage).
 */

const NAME_MAX_LENGTH = 64;
const DESCRIPTION_MAX_LENGTH = 1024;
const SUBMIT_ERROR_MESSAGE =
  'Не удалось сохранить изменения. Проверьте соединение и попробуйте ещё раз';

export type PropertyEditScreenProps = {
  readonly propertyId: string;
};

export function PropertyEditScreen({ propertyId }: PropertyEditScreenProps): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const updateProperty = useUpdateProperty();
  const uploadPhoto = useUploadPropertyPhoto();
  const deletePhoto = useDeletePropertyPhoto();
  // Экран правки — полный экран формы: TabBar глушится, пока внизу
  // смонтирован StickyBottomBar; само подавление делает его монтаж.
  useTabBarSuppression();

  const [draft, setDraft] = useState<PropertyEditDraft | null>(null);
  const [draftOf, setDraftOf] = useState<string | null>(null);
  const [attrsTouched, setAttrsTouched] = useState<{ type: PropertyType | undefined; touched: boolean }>({
    type: undefined,
    touched: false,
  });
  const [serverAttrErrors, setServerAttrErrors] = useState<AttrErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [addressSearchOpen, setAddressSearchOpen] = useState(false);
  // Изменение фото — часть черновика (решение владельца 08.10): stage
  // применяется сабмитом, уход без сохранения его просто умирает. Превью
  // staged-замены — data URL (CSP img-src 'self' data:, ADR 0065): обычная
  // строка в состоянии, никакой дополнительной механики освобождения.
  const [photoStage, setPhotoStage] = useState<PropertyPhotoStage | null>(null);

  const handlePhotoFile = (file: File): void => {
    void readPhotoDataUrl(file).then((previewUrl) => {
      if (previewUrl !== null) {
        setPhotoStage({ kind: 'file', file, previewUrl });
      }
    });
  };

  const handlePhotoRemove = (): void => {
    setPhotoStage({ kind: 'remove' });
  };

  // Черновик инициализируется один раз от загруженного объекта — правки
  // не затираются фоновым рефетчем (паттерн прежней формы).
  const property = propertyQuery.data;
  if (property !== undefined && draftOf !== property.id) {
    setDraft(initialPropertyEditDraft(property));
    setDraftOf(property.id);
  }

  const isSubmitting = updateProperty.isPending;
  // Фото-мутации сабмита в полёте — сохранение глушится целиком (один
  // photo_key у объекта, параллельные загрузка и удаление недопустимы).
  const photoBusy = uploadPhoto.isPending || deletePhoto.isPending;

  // Название необязательно (#1001): очищенное при сохранении
  // регенерирует бэк из типа.
  const ready =
    draft !== null
    && draft.type !== undefined
    && draft.address.trim().length > 0;

  // Правка — полей или фото: и то и другое применяется кнопками
  // сохранения (решение владельца 08.10).
  const fieldsDirty =
    property !== undefined && draft !== null
      ? propertyEditDirty(draft, property, attributeCatalog)
      : false;
  const dirty = fieldsDirty || photoStage !== null;

  const canSubmit = ready && dirty && !isSubmitting && !photoBusy;

  const updateDraft = (patch: Partial<PropertyEditDraft>): void => {
    setDraft((prev) => (prev === null ? prev : { ...prev, ...patch }));
  };

  const changeType = (type: PropertyType): void => {
    // Данные не удаляются — чужие ключи остаются в состоянии (lossless),
    // в payload идут только ключи нового типа; скрытие заполненного —
    // тихое (решение владельца, визард). Ошибки прошлого типа неактуальны.
    updateDraft({ type });
    setAttrsTouched({ type, touched: false });
    setServerAttrErrors({});
  };

  if (propertyQuery.isError) {
    return (
      <>
        <EditChrome propertyId={propertyId} onSave={undefined} />
        <PageContent>
          <div className="flex flex-col items-center gap-4 pt-16 text-center">
            <p className="max-w-[280px] text-sm leading-4 text-content-secondary">
              Не удалось загрузить объект. Проверьте соединение и попробуйте снова
            </p>
            <Button
              variant="secondary"
              size="small"
              onClick={() => void propertyQuery.refetch()}
              disabled={propertyQuery.isFetching}
            >
              Повторить
            </Button>
          </div>
        </PageContent>
      </>
    );
  }

  if (propertyQuery.isPending || property === undefined || draft === null) {
    return <PropertyEditLoading />;
  }

  // Смотрящему и архиву формы не показываем вовсе (как у правки платежей,
  // #607): сервер всё равно ответил бы отказом, а тост «проверьте
  // соединение» вводил бы в заблуждение (обход #758).
  if (!propertyPermissions(property).canEdit) {
    return (
      <>
        <TopNav
          leading={
            <IconButton
              icon={<Cancel />}
              label="Закрыть"
              onClick={() => goBack(router, ROUTES.property(propertyId))}
            />
          }
        >
          <TopNavTitle title="Редактирование объекта" />
        </TopNav>
        <PageContent>
          <div className="flex flex-col items-center gap-3 px-6 pt-16 text-center">
            <h2 className="text-xl font-semibold leading-6 text-content">Правка недоступна</h2>
            <p className="max-w-[360px] text-base leading-[18px] text-content-secondary">
              {property.status === 'archived'
                ? 'Объект в архиве — его можно только смотреть'
                : 'У вас доступ только для просмотра этого объекта'}
            </p>
          </div>
        </PageContent>
      </>
    );
  }

  return (
    <>
      <EditChrome propertyId={propertyId} onSave={canSubmit ? () => void handleSubmit() : undefined} />
      <PageContent>
        <form
          className="flex flex-col gap-8 px-6 pt-6"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSubmit();
          }}
        >
          {/* Фото-слот (Figma 1550:95852, тикет #1228, ADR 0065): глиф
              по типу черновика либо фото. Изменение фото — часть
              черновика (stage), применяется кнопками сохранения вместе
              с полями; уход без сохранения сервер не трогает
              (решение владельца 08.10). */}
          <PropertyPhotoSlot
            propertyId={propertyId}
            photoUrl={property.photoUrl}
            stage={photoStage}
            busy={photoBusy}
            type={draft.type}
            onFileChosen={handlePhotoFile}
            onRemove={handlePhotoRemove}
          />
          <PropertyTypePicker
            title="Тип объекта"
            placeholder="Выберите тип"
            value={draft.type}
            onValueChange={changeType}
          />
          <TextField
            title="Адрес"
            value={draft.address}
            placeholder="Укажите адрес"
            readOnly
            // Поле-триггер: правка — в полноэкранном поиске адреса.
            onClick={() => setAddressSearchOpen(true)}
            onFocus={(event) => event.currentTarget.blur()}
          />
          <TextField
            title="Название объекта"
            placeholder={propertyAutonamePhrase(draft.type)}
            maxLength={NAME_MAX_LENGTH}
            value={draft.name}
            onChange={(event) => updateDraft({ name: event.currentTarget.value })}
          />
          {draft.type !== undefined && (
            <PropertyHousingTypeChips type={draft.type} onChange={changeType} />
          )}
          {draft.type !== undefined && (
            <PropertyCatalogFields
              type={draft.type}
              attributes={draft.attributes}
              onChange={(attributes) => updateDraft({ attributes })}
              showErrors={
                attrsTouched.type === draft.type && attrsTouched.touched
              }
              onFieldsBlur={() => setAttrsTouched({ type: draft.type, touched: true })}
              extraErrors={serverAttrErrors}
              clearable
            />
          )}
          <TextField
            title="Описание"
            multiline
            // Статичный бокс на 8 строк (кадр 1218-54295, #1080): без
            // autoGrow, текст сверх восьми строк скроллится внутри.
            rows={8}
            maxLength={DESCRIPTION_MAX_LENGTH}
            value={draft.description}
            onChange={(event) => updateDraft({ description: event.currentTarget.value })}
          />
        </form>
      </PageContent>
      <StickyBottomBar>
        <div className="flex flex-col gap-3">
          {submitError !== null && (
            <div
              role="alert"
              className="rounded-button bg-surface-danger px-4 py-3 text-sm leading-4 text-danger"
            >
              {submitError}
            </div>
          )}
          <Button
            className="w-full"
            disabled={!canSubmit}
            loading={isSubmitting}
            onClick={() => void handleSubmit()}
          >
            Сохранить изменения
          </Button>
        </div>
      </StickyBottomBar>
      {addressSearchOpen && (
        <PropertyAddressSearch
          value={draft.address}
          onChange={(address) => updateDraft({ address })}
          onSelect={(address) => {
            updateDraft({ address });
            setAddressSearchOpen(false);
          }}
          onClose={() => setAddressSearchOpen(false)}
        />
      )}
    </>
  );

  async function handleSubmit(): Promise<void> {
    if (draft === null || property === undefined) {
      return;
    }
    if (!canSubmit) {
      return;
    }
    const command = buildPropertyEditCommand(draft, attributeCatalog);
    if (command === undefined) {
      return;
    }
    setSubmitError(null);
    // Поля и фото уходят параллельно, каждый частью своей правки: не-dirty
    // часть сети не касается. Каждый сбой — свой сигнал (поля — ошибки
    // атрибутов и плашка, фото — тост сценария), применённая часть гасит
    // свою грязь; назад уходим, только когда сохранено всё.
    const fieldsWork: Promise<boolean> | null = propertyEditDirty(draft, property, attributeCatalog)
      ? updateProperty
          .mutateAsync({ id: propertyId, data: command })
          .then(
            () => true,
            (error: unknown) => {
              if (error instanceof ApiError && error.fieldErrors !== undefined && error.fieldErrors.length > 0) {
                const mapped: AttrErrors = {};
                for (const fe of error.fieldErrors) {
                  mapped[fe.field as AttrKey] = fe.detail;
                }
                setServerAttrErrors(mapped);
              }
              setSubmitError(SUBMIT_ERROR_MESSAGE);
              return false;
            },
          )
      : null;
    const photoWork: Promise<boolean> | null =
      photoStage?.kind === 'file'
        ? uploadPhoto
            .mutateAsync({ id: propertyId, file: photoStage.file })
            .then(
              () => true,
              (error: unknown) => {
                notify.scenarios.property.photoUpdateError(error);
                return false;
              },
            )
        : photoStage?.kind === 'remove'
          ? deletePhoto
              .mutateAsync({ id: propertyId })
              .then(
                () => true,
                (error: unknown) => {
                  notify.scenarios.property.photoDeleteError(error);
                  return false;
                },
              )
          : null;

    const [fieldsSaved = true, photoSaved = true] = await Promise.all([
      fieldsWork ?? Promise.resolve(true),
      photoWork ?? Promise.resolve(true),
    ]);

    if (photoSaved && photoStage !== null) {
      setPhotoStage(null);
    }
    if (fieldsSaved && photoSaved) {
      notify.scenarios.property.updated();
      goBack(router, ROUTES.property(propertyId));
    }
  }
}

/** Хедер экрана правки (Figma 1550:95854): крестик слева, заголовок по
 * центру, галочка справа — активна только при готовой правке. */
function EditChrome({
  propertyId,
  onSave,
}: {
  readonly propertyId: string;
  readonly onSave: (() => void) | undefined;
}): JSX.Element {
  const router = useRouter();
  return (
    <TopNav
      leading={
        <IconButton
          icon={<Cancel />}
          label="Закрыть"
          onClick={() => goBack(router, ROUTES.property(propertyId))}
        />
      }
      trailing={
        <IconButton
          icon={<Check className="h-6 w-6" />}
          label="Сохранить"
          disabled={onSave === undefined}
          onClick={onSave}
        />
      }
    >
      <TopNavTitle title="Редактирование объекта" />
    </TopNav>
  );
}

/** Скелетон формы (§7): фото-круг и поля формой через Skeleton. */
function PropertyEditLoading(): JSX.Element {
  return (
    <>
      <TopNav>
        <TopNavTitle title="Редактирование объекта" />
      </TopNav>
      <PageContent>
        <div className="flex flex-col gap-8 px-6 pt-6" aria-hidden>
          <Skeleton className="mx-auto h-24 w-24 rounded-pill" />
          <Skeleton className="h-[78px] w-full" />
          <Skeleton className="h-[78px] w-full" />
          <Skeleton className="h-[78px] w-full" />
          <div className="flex flex-wrap gap-2">
            <Skeleton className="h-8 w-24 rounded-button" />
            <Skeleton className="h-8 w-32 rounded-button" />
            <Skeleton className="h-8 w-28 rounded-button" />
          </div>
          <Skeleton className="h-[78px] w-full" />
        </div>
      </PageContent>
    </>
  );
}
