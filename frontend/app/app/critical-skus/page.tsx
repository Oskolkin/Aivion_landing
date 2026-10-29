import CriticalSKUsScreen from "@/components/critical-skus-screen";

type CriticalSKUsPageProps = {
  searchParams?: Promise<{ as_of_date?: string }>;
};

export default async function CriticalSKUsPage({ searchParams }: CriticalSKUsPageProps) {
  const params = searchParams ? await searchParams : undefined;
  const asOfDate = params?.as_of_date;

  return <CriticalSKUsScreen initialAsOfDate={asOfDate} />;
}
