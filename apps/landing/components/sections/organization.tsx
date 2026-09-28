import { CardTrio } from "@/components/sections/card-trio";
import orgContacts from "@/assets/sections/org-contacts.webp";
import orgStatuses1 from "@/assets/sections/org-statuses-1.webp";
import orgStatuses2 from "@/assets/sections/org-statuses-2.webp";
import orgTasks1 from "@/assets/sections/org-tasks-1.webp";
import orgTasks2 from "@/assets/sections/org-tasks-2.webp";

// «Организуйте дела» — макет 2814-937 (тот же паттерн трио, что и финансы).
export function Organization() {
  return (
    <CardTrio
      id="organization"
      title="Организуйте дела"
      cards={[
        {
          title: "Записывайте контакты",
          text: "Телефоны арендаторов, мастеров и управляющих",
          images: [
            {
              src: orgContacts,
              alt: "Контакты в Рентли",
            },
          ],
        },
        {
          title: "Создавайте задачи",
          text: "Напоминания о звонках, ремонте и важных делах",
          images: [
            {
              src: orgTasks1,
              alt: "Задача в Рентли",
            },
            {
              src: orgTasks2,
              alt: "Список задач в Рентли",
            },
          ],
        },
        {
          title: "Отслеживайте статусы объектов",
          text: "Сколько месяцев до конца аренды или ремонт объекта",
          images: [
            {
              src: orgStatuses1,
              alt: "Статус объекта в Рентли",
            },
            {
              src: orgStatuses2,
              alt: "Статусы объектов в Рентли",
            },
          ],
        },
      ]}
    />
  );
}
