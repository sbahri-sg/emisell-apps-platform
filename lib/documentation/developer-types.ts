export type DeveloperContractId = 'provider';
export type DeveloperChapter = {
  id: string;
  title: string;
  label: string;
  status: 'guide' | 'mixed' | 'planned' | 'pilot';
  summary: string;
  purpose: string;
  prerequisites: string[];
  sections: {
    id: string;
    title: string;
    paragraphs?: string[];
    steps?: string[];
    code?: { label: string; text: string };
    links?: { label: string; href: string }[];
    api?: { contract: DeveloperContractId; operationId: string; label: string }[];
  }[];
  verify: string[];
  limits: string[];
  catalog?: 'scopes' | 'webhooks' | 'extensions';
};
export type DeveloperGuide = { title: string; chapters: DeveloperChapter[] };
