import { CardTrio } from "@/components/sections/card-trio";
import financeAccounting from "@/assets/sections/finance-accounting.webp";
import financeExpenses from "@/assets/sections/finance-expenses.webp";
import financeRegular1 from "@/assets/sections/finance-regular-1.webp";
import financeRegular2 from "@/assets/sections/finance-regular-2.webp";

// «Управляйте финансами» — макет 2814-923.
export function Finance() {
  return (
    <CardTrio
      id="finance"
      title="Управляйте финансами"
      cards={[
        {
          title: "Отслеживайте регулярные платежи",
          text: "Добавляйте коммунальные услуги, кредит, взносы и другие расходы",
          images: [
            {
              src: financeRegular1,
              alt: "Список регулярных платежей в Рентли",
            },
            {
              src: financeRegular2,
              alt: "Платеж в Рентли",
            },
          ],
        },
        {
          title: "Ведите финансовый учет",
          text: "Отслеживайте финансы по каждому объекту в одном месте",
          images: [
            {
              src: financeAccounting,
              alt: "Финансы объекта в Рентли",
            },
          ],
        },
        {
          title: "Считайте расходы",
          text: "Отмечайте плановые и внезапные расходы",
          images: [
            {
              src: financeExpenses,
              alt: "Расходы в Рентли",
            },
          ],
        },
      ]}
    />
  );
}
