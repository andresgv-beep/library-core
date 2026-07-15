// Fachada legacy: conserva los imports antiguos mientras la UI migra a clientes
// por dominio. Nuevos componentes deberian importar desde los modulos concretos.

export * from './libraryApi.js';
export * from './readerStateApi.js';
export * from './personalDownloadsApi.js';
