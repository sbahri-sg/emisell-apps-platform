import { index, route, type RouteConfig } from '@react-router/dev/routes';

export default [index('routes/home.tsx'), route('products', 'routes/products.tsx'), route('resources/:kind','routes/resources.tsx')] satisfies RouteConfig;
