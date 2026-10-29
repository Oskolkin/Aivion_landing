import StocksReplenishmentScreen from "@/components/stocks-replenishment-screen";

type StocksReplenishmentPageProps = {
  searchParams?: Promise<{ as_of_date?: string }>;
};

export default async function StocksReplenishmentPage({
  searchParams,
}: StocksReplenishmentPageProps) {
  const params = searchParams ? await searchParams : undefined;
  const asOfDate = params?.as_of_date;

  return <StocksReplenishmentScreen initialAsOfDate={asOfDate} />;
}
