import type { FC, SVGProps } from 'react';
import {
  BoldBell,
  BoldBill,
  BoldBox,
  BoldBroom,
  BoldBuild,
  BoldCalculator,
  BoldCar,
  BoldClock,
  BoldCoins,
  BoldCourt,
  BoldCredit,
  BoldFence,
  BoldFridge,
  BoldGas,
  BoldHammer,
  BoldHandCoin,
  BoldHeartBroken,
  BoldHome,
  BoldInternet,
  BoldKey,
  BoldLeaf,
  BoldLight,
  BoldMegaphone,
  BoldMoneyLock,
  BoldOther,
  BoldPaintRoller,
  BoldPercent,
  BoldPerson,
  BoldPipeline,
  BoldPrinter,
  BoldSecurity,
  BoldShield,
  BoldSofa,
  BoldStamp,
  BoldTemperature,
  BoldTrash,
  BoldTv,
  BoldWarning,
  BoldWater,
  BoldWrench,
} from '@/shared/assets/icons';

type IconComponent = FC<SVGProps<SVGSVGElement>>;

/**
 * Реестр имён bold-иконок каталога (#447) → компоненты ассетов
 * (`fill="currentColor"`). Ключи совпадают с полем `icon` catalog.json;
 * покрытие каталога реестром охраняет icon-registry.test.ts — незнакомое
 * имя молча не падает в дефолт пользовательской категории.
 */
export const categoryIconComponents: Readonly<Record<string, IconComponent>> = {
  'bold-bell': BoldBell,
  'bold-bill': BoldBill,
  'bold-box': BoldBox,
  'bold-broom': BoldBroom,
  'bold-build': BoldBuild,
  'bold-calculator': BoldCalculator,
  'bold-car': BoldCar,
  'bold-clock': BoldClock,
  'bold-coins': BoldCoins,
  'bold-court': BoldCourt,
  'bold-credit': BoldCredit,
  'bold-fence': BoldFence,
  'bold-fridge': BoldFridge,
  'bold-gas': BoldGas,
  'bold-hammer': BoldHammer,
  'bold-hand-coin': BoldHandCoin,
  'bold-heart-broken': BoldHeartBroken,
  'bold-home': BoldHome,
  'bold-internet': BoldInternet,
  'bold-key': BoldKey,
  'bold-leaf': BoldLeaf,
  'bold-light': BoldLight,
  'bold-megaphone': BoldMegaphone,
  'bold-money-lock': BoldMoneyLock,
  'bold-other': BoldOther,
  'bold-paint-roller': BoldPaintRoller,
  'bold-percent': BoldPercent,
  'bold-person': BoldPerson,
  'bold-pipeline': BoldPipeline,
  'bold-printer': BoldPrinter,
  'bold-security': BoldSecurity,
  'bold-shield': BoldShield,
  'bold-sofa': BoldSofa,
  'bold-stamp': BoldStamp,
  'bold-temperature': BoldTemperature,
  'bold-trash': BoldTrash,
  'bold-tv': BoldTv,
  'bold-warning': BoldWarning,
  'bold-water': BoldWater,
  'bold-wrench': BoldWrench,
};

/** Компонент иконки по имени из каталога; неизвестное имя — дефолт «прочее». */
export function categoryIconComponent(name: string): IconComponent | undefined {
  return categoryIconComponents[name];
}
